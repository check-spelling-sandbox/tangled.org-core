package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/go-chi/chi/v5"
	"github.com/posthog/posthog-go"
	"tangled.org/core/api/tangled"
	"tangled.org/core/appview/db"
	"tangled.org/core/appview/models"
	"tangled.org/core/consts"
	"tangled.org/core/idresolver"
	"tangled.org/core/orm"
	"tangled.org/core/tid"
)

func (o *OAuth) Router() http.Handler {
	r := chi.NewRouter()

	r.Get("/oauth/client-metadata.json", o.clientMetadata)
	r.Get("/oauth/jwks.json", o.jwks)
	r.Get("/oauth/callback", o.callback)
	return r
}

func (o *OAuth) clientMetadata(w http.ResponseWriter, r *http.Request) {
	doc := o.ClientApp.Config.ClientMetadata()
	doc.JWKSURI = &o.JwksUri
	doc.ClientName = &o.ClientName
	doc.ClientURI = &o.ClientUri

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (o *OAuth) jwks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body := o.ClientApp.Config.PublicJWKS()
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (o *OAuth) callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := o.Logger.With("query", r.URL.Query())

	authReturn := o.GetAuthReturn(r)
	_ = o.ClearAuthReturn(w, r)

	sessData, err := o.ClientApp.ProcessCallback(ctx, r.URL.Query())
	if err != nil {
		var callbackErr *oauth.AuthRequestCallbackError
		if errors.As(err, &callbackErr) {
			l.Debug("callback error", "err", callbackErr)
			http.Redirect(w, r, fmt.Sprintf("/login?error=%s", callbackErr.ErrorCode), http.StatusFound)
			return
		}
		l.Error("failed to process callback", "err", err)
		http.Redirect(w, r, "/login?error=oauth", http.StatusFound)
		return
	}

	if err := o.SaveSession(w, r, sessData); err != nil {
		l.Error("failed to save session", "data", sessData, "err", err)
		errorCode := "session"
		if errors.Is(err, ErrMaxAccountsReached) {
			errorCode = "max_accounts"
		}
		http.Redirect(w, r, fmt.Sprintf("/login?error=%s", errorCode), http.StatusFound)
		return
	}

	o.Logger.Debug("session saved successfully")

	go o.addToDefaultKnot(sessData.AccountDID.String())
	go o.addToDefaultSpindle(sessData.AccountDID.String())
	go o.ensureTangledProfile(sessData)

	if !o.Config.Core.Dev {
		err = o.Posthog.Enqueue(posthog.Capture{
			DistinctId: sessData.AccountDID.String(),
			Event:      "signin",
		})
		if err != nil {
			o.Logger.Error("failed to enqueue posthog event", "err", err)
		}
	}

	redirectURL := "/"
	if authReturn.ReturnURL != "" {
		redirectURL = authReturn.ReturnURL
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (o *OAuth) addToDefaultSpindle(did string) {
	l := o.Logger.With("subject", did)

	// use the tangled.sh app password to get an accessJwt
	// and create an sh.tangled.spindle.member record with that
	spindleMembers, err := db.GetSpindleMembers(
		o.Db,
		orm.FilterEq("instance", "spindle.tangled.sh"),
		orm.FilterEq("subject", did),
	)
	if err != nil {
		l.Error("failed to get spindle members", "err", err)
		return
	}

	if len(spindleMembers) != 0 {
		l.Warn("already a member of the default spindle")
		return
	}

	l.Debug("adding to default spindle")
	session, err := CreateAppPasswordSession(o.IdResolver, o.Config.Core.AppPassword, consts.TangledDid)
	if err != nil {
		l.Error("failed to create session", "err", err)
		return
	}

	record := tangled.SpindleMember{
		LexiconTypeID: "sh.tangled.spindle.member",
		Subject:       did,
		Instance:      consts.DefaultSpindle,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	if err := session.putRecord(record, tangled.SpindleMemberNSID); err != nil {
		l.Error("failed to add to default spindle", "err", err)
		return
	}

	l.Debug("successfully added to default spindle", "did", did)
}

func (o *OAuth) addToDefaultKnot(did string) {
	l := o.Logger.With("subject", did)

	// use the tangled.sh app password to get an accessJwt
	// and create an sh.tangled.spindle.member record with that

	allKnots, err := o.Enforcer.GetKnotsForUser(did)
	if err != nil {
		l.Error("failed to get knot members for did", "err", err)
		return
	}

	if slices.Contains(allKnots, consts.DefaultKnot) {
		l.Warn("already a member of the default knot")
		return
	}

	l.Debug("adding to default knot")
	session, err := CreateAppPasswordSession(o.IdResolver, o.Config.Core.TmpAltAppPassword, consts.IcyDid)
	if err != nil {
		l.Error("failed to create session", "err", err)
		return
	}

	record := tangled.KnotMember{
		LexiconTypeID: "sh.tangled.knot.member",
		Subject:       did,
		Domain:        consts.DefaultKnot,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	if err := session.putRecord(record, tangled.KnotMemberNSID); err != nil {
		l.Error("failed to add to default knot", "err", err)
		return
	}

	if err := o.Enforcer.AddKnotMember(consts.DefaultKnot, did); err != nil {
		l.Error("failed to set up enforcer rules", "err", err)
		return
	}

	l.Debug("successfully addeds to default Knot")
}

func (o *OAuth) ensureTangledProfile(sessData *oauth.ClientSessionData) {
	ctx := context.Background()
	did := sessData.AccountDID.String()
	l := o.Logger.With("did", did)

	profile, _ := db.GetProfile(o.Db, did)
	if profile != nil {
		l.Debug("profile already exists in DB")
		return
	}

	l.Debug("creating empty Tangled profile")

	sess, err := o.ClientApp.ResumeSession(ctx, sessData.AccountDID, sessData.SessionID)
	if err != nil {
		l.Error("failed to resume session for profile creation", "err", err)
		return
	}
	client := sess.APIClient()

	_, err = comatproto.RepoPutRecord(ctx, client, &comatproto.RepoPutRecord_Input{
		Collection: tangled.ActorProfileNSID,
		Repo:       did,
		Rkey:       "self",
		Record:     &lexutil.LexiconTypeDecoder{Val: &tangled.ActorProfile{}},
	})

	if err != nil {
		l.Error("failed to create empty profile on PDS", "err", err)
		return
	}

	tx, err := o.Db.BeginTx(ctx, nil)
	if err != nil {
		l.Error("failed to start transaction", "err", err)
		return
	}

	emptyProfile := &models.Profile{Did: did}
	if err := db.UpsertProfile(tx, emptyProfile); err != nil {
		l.Error("failed to create empty profile in DB", "err", err)
		return
	}

	l.Debug("successfully created empty Tangled profile on PDS and DB")
}

// create a AppPasswordSession using apppasswords
type AppPasswordSession struct {
	AccessJwt   string `json:"accessJwt"`
	PdsEndpoint string
	Did         string
}

func CreateAppPasswordSession(res *idresolver.Resolver, appPassword, did string) (*AppPasswordSession, error) {
	if appPassword == "" {
		return nil, fmt.Errorf("no app password configured")
	}

	resolved, err := res.ResolveIdent(context.Background(), did)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tangled.sh DID %s: %v", did, err)
	}

	pdsEndpoint := resolved.PDSEndpoint()
	if pdsEndpoint == "" {
		return nil, fmt.Errorf("no PDS endpoint found for tangled.sh DID %s", did)
	}

	sessionPayload := map[string]string{
		"identifier": did,
		"password":   appPassword,
	}
	sessionBytes, err := json.Marshal(sessionPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session payload: %v", err)
	}

	sessionURL := pdsEndpoint + "/xrpc/com.atproto.server.createSession"
	sessionReq, err := http.NewRequestWithContext(context.Background(), "POST", sessionURL, bytes.NewBuffer(sessionBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create session request: %v", err)
	}
	sessionReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	sessionResp, err := client.Do(sessionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	defer sessionResp.Body.Close()

	if sessionResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create session: HTTP %d", sessionResp.StatusCode)
	}

	var session AppPasswordSession
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session response: %v", err)
	}

	session.PdsEndpoint = pdsEndpoint
	session.Did = did

	return &session, nil
}

func (s *AppPasswordSession) putRecord(record any, collection string) error {
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal knot member record: %w", err)
	}

	payload := map[string]any{
		"repo":       s.Did,
		"collection": collection,
		"rkey":       tid.TID(),
		"record":     json.RawMessage(recordBytes),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request payload: %w", err)
	}

	url := s.PdsEndpoint + "/xrpc/com.atproto.repo.putRecord"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.AccessJwt)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add user to default service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add user to default service: HTTP %d", resp.StatusCode)
	}

	return nil
}
