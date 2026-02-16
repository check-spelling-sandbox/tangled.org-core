import {
  LocalActorResolver,
  CompositeHandleResolver,
  DohJsonHandleResolver,
  WellKnownHandleResolver,
  CompositeDidDocumentResolver,
  PlcDidDocumentResolver,
  WebDidDocumentResolver,
} from "@atcute/identity-resolver";

// Initialize resolvers for Cloudflare Workers
const handleResolver = new CompositeHandleResolver({
  strategy: "race",
  methods: {
    dns: new DohJsonHandleResolver({
      dohUrl: "https://cloudflare-dns.com/dns-query",
    }),
    http: new WellKnownHandleResolver(),
  },
});

const didDocumentResolver = new CompositeDidDocumentResolver({
  methods: {
    plc: new PlcDidDocumentResolver(),
    web: new WebDidDocumentResolver(),
  },
});

const actorResolver = new LocalActorResolver({
  handleResolver,
  didDocumentResolver,
});

export default {
  async fetch(request, env) {
    // Helper function to generate a color from a string
    const stringToColor = (str) => {
      let hash = 0;
      for (let i = 0; i < str.length; i++) {
        hash = str.charCodeAt(i) + ((hash << 5) - hash);
      }
      let color = "#";
      for (let i = 0; i < 3; i++) {
        const value = (hash >> (i * 8)) & 0xff;
        color += ("00" + value.toString(16)).substr(-2);
      }
      return color;
    };

    // Helper function to fetch Tangled profile from PDS
    const getTangledAvatarFromPDS = async (actor) => {
      try {
        // Resolve the identity
        const identity = await actorResolver.resolve(actor);
        if (!identity) {
          console.log({
            level: "debug",
            message: "failed to resolve identity",
            actor: actor,
          });
          return null;
        }

        const did = identity.did;
        const pdsEndpoint = identity.pds.replace(/\/$/, ""); // Remove trailing slash

        if (!pdsEndpoint) {
          console.log({
            level: "debug",
            message: "no PDS endpoint found",
            actor: actor,
            did: did,
          });
          return null;
        }

        const profileUrl = `${pdsEndpoint}/xrpc/com.atproto.repo.getRecord?repo=${did}&collection=sh.tangled.actor.profile&rkey=self`;

        // Fetch the Tangled profile record from PDS
        const profileResponse = await fetch(profileUrl);

        if (!profileResponse.ok) {
          console.log({
            level: "debug",
            message: "no Tangled profile found on PDS",
            actor: actor,
            status: profileResponse.status,
          });
          return null;
        }

        const profileData = await profileResponse.json();
        const avatarBlob = profileData?.value?.avatar;

        if (!avatarBlob) {
          console.log({
            level: "debug",
            message: "Tangled profile has no avatar",
            actor: actor,
          });
          return null;
        }

        // Extract CID from blob reference object
        // The ref might be an object with $link property or a string
        let avatarCID;
        if (typeof avatarBlob.ref === "string") {
          avatarCID = avatarBlob.ref;
        } else if (avatarBlob.ref?.$link) {
          avatarCID = avatarBlob.ref.$link;
        } else if (typeof avatarBlob === "string") {
          avatarCID = avatarBlob;
        }

        if (!avatarCID || typeof avatarCID !== "string") {
          console.log({
            level: "warn",
            message: "could not extract valid CID from avatar blob",
            actor: actor,
            avatarBlob: avatarBlob,
            avatarBlobRef: avatarBlob.ref,
          });
          return null;
        }

        // Construct blob URL (pdsEndpoint already has trailing slash removed)
        const blobUrl = `${pdsEndpoint}/xrpc/com.atproto.sync.getBlob?did=${did}&cid=${avatarCID}`;

        return blobUrl;
      } catch (e) {
        console.log({
          level: "warn",
          message: "error fetching Tangled avatar from PDS",
          actor: actor,
          error: e.message,
        });
        return null;
      }
    };

    const url = new URL(request.url);
    const { pathname, searchParams } = url;

    if (!pathname || pathname === "/") {
      return new Response(
        `This is Tangled's avatar service. It fetches your pretty avatar from your PDS, Bluesky, or generates a placeholder.
You can't use this directly unfortunately since all requests are signed and may only originate from the appview.`,
      );
    }

    const size = searchParams.get("size");
    const resizeToTiny = size === "tiny";

    const cache = caches.default;
    let cacheKey = request.url;
    let response = await cache.match(cacheKey);
    if (response) return response;

    const pathParts = pathname.slice(1).split("/");
    if (pathParts.length < 2) {
      return new Response("Bad URL", { status: 400 });
    }

    const [signatureHex, actor] = pathParts;
    const actorBytes = new TextEncoder().encode(actor);

    const key = await crypto.subtle.importKey(
      "raw",
      new TextEncoder().encode(env.AVATAR_SHARED_SECRET),
      { name: "HMAC", hash: "SHA-256" },
      false,
      ["sign", "verify"],
    );

    const computedSigBuffer = await crypto.subtle.sign("HMAC", key, actorBytes);
    const computedSig = Array.from(new Uint8Array(computedSigBuffer))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");

    console.log({
      level: "debug",
      message: "avatar request for: " + actor,
      computedSignature: computedSig,
      providedSignature: signatureHex,
    });

    const sigBytes = Uint8Array.from(
      signatureHex.match(/.{2}/g).map((b) => parseInt(b, 16)),
    );
    const valid = await crypto.subtle.verify("HMAC", key, sigBytes, actorBytes);

    if (!valid) {
      return new Response("Invalid signature", { status: 403 });
    }

    try {
      let avatarUrl = null;

      // Try to get Tangled avatar from user's PDS first
      avatarUrl = await getTangledAvatarFromPDS(actor);

      // If no Tangled avatar, fall back to Bluesky
      if (!avatarUrl) {
        console.log({
          level: "debug",
          message: "no Tangled avatar, falling back to Bluesky",
          actor: actor,
        });

        const profileResponse = await fetch(
          `https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=${actor}`,
        );

        if (profileResponse.ok) {
          const profile = await profileResponse.json();
          avatarUrl = profile.avatar;
        }
      }

      if (!avatarUrl) {
        // Generate a random color based on the actor string
        console.log({
          level: "debug",
          message: "no avatar found, generating placeholder",
          actor: actor,
        });

        const bgColor = stringToColor(actor);
        const size = resizeToTiny ? 32 : 128;
        const svg = `<svg width="${size}" height="${size}" viewBox="0 0 ${size} ${size}" xmlns="http://www.w3.org/2000/svg"><rect width="${size}" height="${size}" fill="${bgColor}"/></svg>`;
        const svgData = new TextEncoder().encode(svg);

        response = new Response(svgData, {
          headers: {
            "Content-Type": "image/svg+xml",
            "Cache-Control": "public, max-age=43200",
          },
        });
        await cache.put(cacheKey, response.clone());
        return response;
      }

      // Fetch and optionally resize the avatar
      let avatarResponse;
      if (resizeToTiny) {
        avatarResponse = await fetch(avatarUrl, {
          // cf: {
          //   image: {
          //     width: 32,
          //     height: 32,
          //     fit: "cover",
          //     format: "webp",
          //   },
          // },
        });
      } else {
        avatarResponse = await fetch(avatarUrl);
      }

      if (!avatarResponse.ok) {
        return new Response(`failed to fetch avatar for ${actor}.`, {
          status: avatarResponse.status,
        });
      }

      const avatarData = await avatarResponse.arrayBuffer();
      const contentType =
        avatarResponse.headers.get("content-type") || "image/jpeg";

      response = new Response(avatarData, {
        headers: {
          "Content-Type": contentType,
          "Cache-Control": "public, max-age=43200",
        },
      });

      await cache.put(cacheKey, response.clone());
      return response;
    } catch (error) {
      return new Response(`error fetching avatar: ${error.message}`, {
        status: 500,
      });
    }
  },
};
