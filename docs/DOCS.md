---
title: Tangled docs
author: The Tangled Contributors
date: 21 Sun, Dec 2025
abstract: |
  Tangled is a decentralized code hosting and collaboration
  platform. Every component of Tangled is open-source and
  self-hostable. [tangled.org](https://tangled.org) also
  provides hosting and CI services that are free to use.

  There are several models for decentralized code
  collaboration platforms, ranging from ActivityPub’s
  (Forgejo) federated model, to Radicle’s entirely P2P model.
  Our approach attempts to be the best of both worlds by
  adopting the AT Protocol—a protocol for building decentralized
  social applications with a central identity

  Our approach to this is the idea of “knots”. Knots are
  lightweight, headless servers that enable users to host Git
  repositories with ease. Knots are designed for either single
  or multi-tenant use which is perfect for self-hosting on a
  Raspberry Pi at home, or larger “community” servers. By
  default, Tangled provides managed knots where you can host
  your repositories for free.

  The appview at tangled.org acts as a consolidated "view"
  into the whole network, allowing users to access, clone and
  contribute to repositories hosted across different knots
  seamlessly.
---

# Quick start guide

## Login or sign up

You can [login](https://tangled.org) by using your AT Protocol
account. If you are unclear on what that means, simply head
to the [signup](https://tangled.org/signup) page and create
an account. By doing so, you will be choosing Tangled as
your account provider (you will be granted a handle of the
form `user.tngl.sh`).

In the AT Protocol network, users are free to choose their account
provider (known as a "Personal Data Service", or PDS), and
login to applications that support AT accounts.

You can think of it as "one account for all of the atmosphere"!

If you already have an AT account (you may have one if you
signed up to Bluesky, for example), you can login with the
same handle on Tangled (so just use `user.bsky.social` on
the login page).

## Add an SSH key

Once you are logged in, you can start creating repositories
and pushing code. Tangled supports pushing git repositories
over SSH.

First, you'll need to generate an SSH key if you don't
already have one:

```bash
ssh-keygen -t ed25519 -C "foo@bar.com"
```

When prompted, save the key to the default location
(`~/.ssh/id_ed25519`) and optionally set a passphrase.

Copy your public key to your clipboard:

```bash
# on X11
cat ~/.ssh/id_ed25519.pub | xclip -sel c

# on wayland
cat ~/.ssh/id_ed25519.pub | wl-copy

# on macos
cat ~/.ssh/id_ed25519.pub | pbcopy
```

Now, navigate to 'Settings' -> 'Keys' and hit 'Add Key',
paste your public key, give it a descriptive name, and hit
save.

## Create a repository

Once your SSH key is added, create your first repository:

1. Hit the green `+` icon on the topbar, and select
   repository
2. Enter a repository name
3. Add a description
4. Choose a knotserver to host this repository on
5. Hit create

Knots are self-hostable, lightweight Git servers that can
host your repository. Unlike traditional code forges, your
code can live on any server. Read the [Knots](TODO) section
for more.

## Configure SSH

To ensure Git uses the correct SSH key and connects smoothly
to Tangled, add this configuration to your `~/.ssh/config`
file:

```
Host tangled.org
    Hostname tangled.org
    User git
    IdentityFile ~/.ssh/id_ed25519
    AddressFamily inet
```

This tells SSH to use your specific key when connecting to
Tangled and prevents authentication issues if you have
multiple SSH keys.

Note that this configuration only works for knotservers that
are hosted by tangled.org. If you use a custom knot, refer
to the [Knots](TODO) section.

## Push your first repository

Initialize a new Git repository:

```bash
mkdir my-project
cd my-project

git init
echo "# My Project" > README.md
```

Add some content and push!

```bash
git add README.md
git commit -m "Initial commit"
git remote add origin git@tangled.org:user.tngl.sh/my-project
git push -u origin main
```

That's it! Your code is now hosted on Tangled.

## Migrating an existing repository

Moving your repositories from GitHub, GitLab, Bitbucket, or
any other Git forge to Tangled is straightforward. You'll
simply change your repository's remote URL. At the moment,
Tangled does not have any tooling to migrate data such as
GitHub issues or pull requests.

First, create a new repository on tangled.org as described
in the [Quick Start Guide](#create-a-repository).

Navigate to your existing local repository:

```bash
cd /path/to/your/existing/repo
```

You can inspect your existing Git remote like so:

```bash
git remote -v
```

You'll see something like:

```
origin  git@github.com:username/my-project (fetch)
origin  git@github.com:username/my-project (push)
```

Update the remote URL to point to tangled:

```bash
git remote set-url origin git@tangled.org:user.tngl.sh/my-project
```

Verify the change:

```bash
git remote -v
```

You should now see:

```
origin  git@tangled.org:user.tngl.sh/my-project (fetch)
origin  git@tangled.org:user.tngl.sh/my-project (push)
```

Push all your branches and tags to Tangled:

```bash
git push -u origin --all
git push -u origin --tags
```

Your repository is now migrated to Tangled! All commit
history, branches, and tags have been preserved.

## Mirroring a repository to Tangled

If you want to maintain your repository on multiple forges
simultaneously, for example, keeping your primary repository
on GitHub while mirroring to Tangled for backup or
redundancy, you can do so by adding multiple remotes.

You can configure your local repository to push to both
Tangled and, say, GitHub. You may already have the following
setup:

```
$ git remote -v
origin  git@github.com:username/my-project (fetch)
origin  git@github.com:username/my-project (push)
```

Now add Tangled as an additional push URL to the same
remote:

```bash
git remote set-url --add --push origin git@tangled.org:user.tngl.sh/my-project
```

You also need to re-add the original URL as a push
destination (Git replaces the push URL when you use `--add`
the first time):

```bash
git remote set-url --add --push origin git@github.com:username/my-project
```

Verify your configuration:

```
$ git remote -v
origin  git@github.com:username/repo (fetch)
origin  git@tangled.org:username/my-project (push)
origin  git@github.com:username/repo (push)
```

Notice that there's one fetch URL (the primary remote) and
two push URLs. Now, whenever you push, Git will
automatically push to both remotes:

```bash
git push origin main
```

This single command pushes your `main` branch to both GitHub
and Tangled simultaneously.

To push all branches and tags:

```bash
git push origin --all
git push origin --tags
```

If you prefer more control over which remote you push to,
you can maintain separate remotes:

```bash
git remote add github git@github.com:username/my-project
git remote add tangled git@tangled.org:username/my-project
```

Then push to each explicitly:

```bash
git push github main
git push tangled main
```

# Knot self-hosting guide

So you want to run your own knot server? Great! Here are a few prerequisites:

1. A server of some kind (a VPS, a Raspberry Pi, etc.). Preferably running a Linux distribution of some kind.
2. A (sub)domain name. People generally use `knot.example.com`.
3. A valid SSL certificate for your domain.

## NixOS

Refer to the [knot
module](https://tangled.org/tangled.org/core/blob/master/nix/modules/knot.nix)
for a full list of options. Sample configurations:

- [The test VM](https://tangled.org/tangled.org/core/blob/master/nix/vm.nix#L85)
- [@pyrox.dev/nix](https://tangled.org/pyrox.dev/nix/blob/d19571cc1b5fe01035e1e6951ec8cf8a476b4dee/hosts/marvin/services/tangled.nix#L15-25)

## Docker

Refer to
[@tangled.org/knot-docker](https://tangled.org/@tangled.org/knot-docker).
Note that this is community maintained.

## Manual setup

First, clone this repository:

```
git clone https://tangled.org/@tangled.org/core
```

Then, build the `knot` CLI. This is the knot administration
and operation tool. For the purpose of this guide, we're
only concerned with these subcommands:

- `knot server`: the main knot server process, typically
  run as a supervised service
- `knot guard`: handles role-based access control for git
  over SSH (you'll never have to run this yourself)
- `knot keys`: fetches SSH keys associated with your knot;
  we'll use this to generate the SSH
  `AuthorizedKeysCommand`

```
cd core
export CGO_ENABLED=1
go build -o knot ./cmd/knot
```

Next, move the `knot` binary to a location owned by `root` --
`/usr/local/bin/` is a good choice. Make sure the binary itself is also owned by `root`:

```
sudo mv knot /usr/local/bin/knot
sudo chown root:root /usr/local/bin/knot
```

This is necessary because SSH `AuthorizedKeysCommand` requires [really
specific permissions](https://stackoverflow.com/a/27638306). The
`AuthorizedKeysCommand` specifies a command that is run by `sshd` to
retrieve a user's public SSH keys dynamically for authentication. Let's
set that up.

```
sudo tee /etc/ssh/sshd_config.d/authorized_keys_command.conf <<EOF
Match User git
  AuthorizedKeysCommand /usr/local/bin/knot keys -o authorized-keys
  AuthorizedKeysCommandUser nobody
EOF
```

Then, reload `sshd`:

```
sudo systemctl reload ssh
```

Next, create the `git` user. We'll use the `git` user's home directory
to store repositories:

```
sudo adduser git
```

Create `/home/git/.knot.env` with the following, updating the values as
necessary. The `KNOT_SERVER_OWNER` should be set to your
DID, you can find your DID in the [Settings](https://tangled.sh/settings) page.

```
KNOT_REPO_SCAN_PATH=/home/git
KNOT_SERVER_HOSTNAME=knot.example.com
APPVIEW_ENDPOINT=https://tangled.org
KNOT_SERVER_OWNER=did:plc:foobar
KNOT_SERVER_INTERNAL_LISTEN_ADDR=127.0.0.1:5444
KNOT_SERVER_LISTEN_ADDR=127.0.0.1:5555
```

If you run a Linux distribution that uses systemd, you can
use the provided service file to run the server. Copy
[`knotserver.service`](https://tangled.org/tangled.org/core/blob/master/systemd/knotserver.service)
to `/etc/systemd/system/`. Then, run:

```
systemctl enable knotserver
systemctl start knotserver
```

The last step is to configure a reverse proxy like Nginx or Caddy to front your
knot. Here's an example configuration for Nginx:

```
server {
    listen 80;
    listen [::]:80;
    server_name knot.example.com;

    location / {
        proxy_pass http://localhost:5555;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # wss endpoint for git events
    location /events {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_set_header Upgrade websocket;
        proxy_set_header Connection Upgrade;
        proxy_pass http://localhost:5555;
    }
  # additional config for SSL/TLS go here.
}

```

Remember to use Let's Encrypt or similar to procure a certificate for your
knot domain.

You should now have a running knot server! You can finalize
your registration by hitting the `verify` button on the
[/settings/knots](https://tangled.org/settings/knots) page. This simply creates
a record on your PDS to announce the existence of the knot.

### Custom paths

(This section applies to manual setup only. Docker users should edit the mounts
in `docker-compose.yml` instead.)

Right now, the database and repositories of your knot lives in `/home/git`. You
can move these paths if you'd like to store them in another folder. Be careful
when adjusting these paths:

- Stop your knot when moving data (e.g. `systemctl stop knotserver`) to prevent
  any possible side effects. Remember to restart it once you're done.
- Make backups before moving in case something goes wrong.
- Make sure the `git` user can read and write from the new paths.

#### Database

As an example, let's say the current database is at `/home/git/knotserver.db`,
and we want to move it to `/home/git/database/knotserver.db`.

Copy the current database to the new location. Make sure to copy the `.db-shm`
and `.db-wal` files if they exist.

```
mkdir /home/git/database
cp /home/git/knotserver.db* /home/git/database
```

In the environment (e.g. `/home/git/.knot.env`), set `KNOT_SERVER_DB_PATH` to
the new file path (_not_ the directory):

```
KNOT_SERVER_DB_PATH=/home/git/database/knotserver.db
```

#### Repositories

As an example, let's say the repositories are currently in `/home/git`, and we
want to move them into `/home/git/repositories`.

Create the new folder, then move the existing repositories (if there are any):

```
mkdir /home/git/repositories
# move all DIDs into the new folder; these will vary for you!
mv /home/git/did:plc:wshs7t2adsemcrrd4snkeqli /home/git/repositories
```

In the environment (e.g. `/home/git/.knot.env`), update `KNOT_REPO_SCAN_PATH`
to the new directory:

```
KNOT_REPO_SCAN_PATH=/home/git/repositories
```

Similarly, update your `sshd` `AuthorizedKeysCommand` to use the updated
repository path:

```
sudo tee /etc/ssh/sshd_config.d/authorized_keys_command.conf <<EOF
Match User git
  AuthorizedKeysCommand /usr/local/bin/knot keys -o authorized-keys -git-dir /home/git/repositories
  AuthorizedKeysCommandUser nobody
EOF
```

Make sure to restart your SSH server!

#### MOTD (message of the day)

To configure the MOTD used ("Welcome to this knot!" by default), edit the
`/home/git/motd` file:

```
printf "Hi from this knot!\n" > /home/git/motd
```

Note that you should add a newline at the end if setting a non-empty message
since the knot won't do this for you.

## Troubleshooting

If you run your own knot, you may run into some of these
common issues. You can always join the
[IRC](https://web.libera.chat/#tangled) or
[Discord](https://chat.tangled.org/) if this section does
not help.

### Unable to push

If you are unable to push to your knot or repository:

1. First, ensure that you have added your SSH public key to
   your account
2. Check to see that your knot has synced the key by running
   `knot keys`
3. Check to see if git is supplying the correct private key
   when pushing: `GIT_SSH_COMMAND="ssh -v" git push ...`
4. Check to see if `sshd` on the knot is rejecting the push
   for some reason: `journalctl -xeu ssh` (or `sshd`,
   depending on your machine). These logs are unavailable if
   using docker.
5. Check to see if the knot itself is rejecting the push,
   depending on your setup, the logs might be in one of the
   following paths:
   - `/tmp/knotguard.log`
   - `/home/git/log`
   - `/home/git/guard.log`

# Spindles

## Pipelines

Spindle workflows allow you to write CI/CD pipelines in a
simple format. They're located in the `.tangled/workflows`
directory at the root of your repository, and are defined
using YAML.

The fields are:

- [Trigger](#trigger): A **required** field that defines
  when a workflow should be triggered.
- [Engine](#engine): A **required** field that defines which
  engine a workflow should run on.
- [Clone options](#clone-options): An **optional** field
  that defines how the repository should be cloned.
- [Dependencies](#dependencies): An **optional** field that
  allows you to list dependencies you may need.
- [Environment](#environment): An **optional** field that
  allows you to define environment variables.
- [Steps](#steps): An **optional** field that allows you to
  define what steps should run in the workflow.

### Trigger

The first thing to add to a workflow is the trigger, which
defines when a workflow runs. This is defined using a `when`
field, which takes in a list of conditions. Each condition
has the following fields:

- `event`: This is a **required** field that defines when
  your workflow should run. It's a list that can take one or
  more of the following values:
  - `push`: The workflow should run every time a commit is
    pushed to the repository.
  - `pull_request`: The workflow should run every time a
    pull request is made or updated.
  - `manual`: The workflow can be triggered manually.
- `branch`: Defines which branches the workflow should run
  for. If used with the `push` event, commits to the
  branch(es) listed here will trigger the workflow. If used
  with the `pull_request` event, updates to pull requests
  targeting the branch(es) listed here will trigger the
  workflow. This field has no effect with the `manual`
  event. Supports glob patterns using `*` and `**` (e.g.,
  `main`, `develop`, `release-*`). Either `branch` or `tag`
  (or both) must be specified for `push` events.
- `tag`: Defines which tags the workflow should run for.
  Only used with the `push` event - when tags matching the
  pattern(s) listed here are pushed, the workflow will
  trigger. This field has no effect with `pull_request` or
  `manual` events. Supports glob patterns using `*` and `**`
  (e.g., `v*`, `v1.*`, `release-**`). Either `branch` or
  `tag` (or both) must be specified for `push` events.

For example, if you'd like to define a workflow that runs
when commits are pushed to the `main` and `develop`
branches, or when pull requests that target the `main`
branch are updated, or manually, you can do so with:

```yaml
when:
  - event: ["push", "manual"]
    branch: ["main", "develop"]
  - event: ["pull_request"]
    branch: ["main"]
```

You can also trigger workflows on tag pushes. For instance,
to run a deployment workflow when tags matching `v*` are
pushed:

```yaml
when:
  - event: ["push"]
    tag: ["v*"]
```

You can even combine branch and tag patterns in a single
constraint (the workflow triggers if either matches):

```yaml
when:
  - event: ["push"]
    branch: ["main", "release-*"]
    tag: ["v*", "stable"]
```

### Engine

Next is the engine on which the workflow should run, defined
using the **required** `engine` field. The currently
supported engines are:

- `nixery`: This uses an instance of
  [Nixery](https://nixery.dev) to run steps, which allows
  you to add [dependencies](#dependencies) from
  Nixpkgs (https://github.com/NixOS/nixpkgs). You can
  search for packages on https://search.nixos.org, and
  there's a pretty good chance the package(s) you're looking
  for will be there.

Example:

```yaml
engine: "nixery"
```

### Clone options

When a workflow starts, the first step is to clone the
repository. You can customize this behavior using the
**optional** `clone` field. It has the following fields:

- `skip`: Setting this to `true` will skip cloning the
  repository. This can be useful if your workflow is doing
  something that doesn't require anything from the
  repository itself. This is `false` by default.
- `depth`: This sets the number of commits, or the "clone
  depth", to fetch from the repository. For example, if you
  set this to 2, the last 2 commits will be fetched. By
  default, the depth is set to 1, meaning only the most
  recent commit will be fetched, which is the commit that
  triggered the workflow.
- `submodules`: If you use Git submodules
  (https://git-scm.com/book/en/v2/Git-Tools-Submodules)
  in your repository, setting this field to `true` will
  recursively fetch all submodules. This is `false` by
  default.

The default settings are:

```yaml
clone:
  skip: false
  depth: 1
  submodules: false
```

### Dependencies

Usually when you're running a workflow, you'll need
additional dependencies. The `dependencies` field lets you
define which dependencies to get, and from where. It's a
key-value map, with the key being the registry to fetch
dependencies from, and the value being the list of
dependencies to fetch.

The registry URL syntax can be found [on the nix
manual](https://nix.dev/manual/nix/2.18/command-ref/new-cli/nix3-registry-add).

Say you want to fetch Node.js and Go from `nixpkgs`, and a
package called `my_pkg` you've made from your own registry
at your repository at
`https://tangled.org/@example.com/my_pkg`. You can define
those dependencies like so:

```yaml
dependencies:
  # nixpkgs
  nixpkgs:
    - nodejs
    - go
  # unstable
  nixpkgs/nixpkgs-unstable:
    - bun
  # custom registry
  git+https://tangled.org/@example.com/my_pkg:
    - my_pkg
```

Now these dependencies are available to use in your
workflow!

### Environment

The `environment` field allows you define environment
variables that will be available throughout the entire
workflow. **Do not put secrets here, these environment
variables are visible to anyone viewing the repository. You
can add secrets for pipelines in your repository's
settings.**

Example:

```yaml
environment:
  GOOS: "linux"
  GOARCH: "arm64"
  NODE_ENV: "production"
  MY_ENV_VAR: "MY_ENV_VALUE"
```

By default, the following environment variables set:

- `CI` - Always set to `true` to indicate a CI environment
- `TANGLED_PIPELINE_ID` - The AT URI of the current pipeline
- `TANGLED_REPO_KNOT` - The repository's knot hostname
- `TANGLED_REPO_DID` - The DID of the repository owner
- `TANGLED_REPO_NAME` - The name of the repository
- `TANGLED_REPO_DEFAULT_BRANCH` - The default branch of the
  repository
- `TANGLED_REPO_URL` - The full URL to the repository

These variables are only available when the pipeline is
triggered by a push:

- `TANGLED_REF` - The full git reference (e.g.,
  `refs/heads/main` or `refs/tags/v1.0.0`)
- `TANGLED_REF_NAME` - The short name of the reference
  (e.g., `main` or `v1.0.0`)
- `TANGLED_REF_TYPE` - The type of reference, either
  `branch` or `tag`
- `TANGLED_SHA` - The commit SHA that triggered the pipeline
- `TANGLED_COMMIT_SHA` - Alias for `TANGLED_SHA`

These variables are only available when the pipeline is
triggered by a pull request:

- `TANGLED_PR_SOURCE_BRANCH` - The source branch of the pull
  request
- `TANGLED_PR_TARGET_BRANCH` - The target branch of the pull
  request
- `TANGLED_PR_SOURCE_SHA` - The commit SHA of the source
  branch

### Steps

The `steps` field allows you to define what steps should run
in the workflow. It's a list of step objects, each with the
following fields:

- `name`: This field allows you to give your step a name.
  This name is visible in your workflow runs, and is used to
  describe what the step is doing.
- `command`: This field allows you to define a command to
  run in that step. The step is run in a Bash shell, and the
  logs from the command will be visible in the pipelines
  page on the Tangled website. The
  [dependencies](#dependencies) you added will be available
  to use here.
- `environment`: Similar to the global
  [environment](#environment) config, this **optional**
  field is a key-value map that allows you to set
  environment variables for the step. **Do not put secrets
  here, these environment variables are visible to anyone
  viewing the repository. You can add secrets for pipelines
  in your repository's settings.**

Example:

```yaml
steps:
  - name: "Build backend"
    command: "go build"
    environment:
      GOOS: "darwin"
      GOARCH: "arm64"
  - name: "Build frontend"
    command: "npm run build"
    environment:
      NODE_ENV: "production"
```

### Complete workflow

```yaml
# .tangled/workflows/build.yml

when:
  - event: ["push", "manual"]
    branch: ["main", "develop"]
  - event: ["pull_request"]
    branch: ["main"]

engine: "nixery"

# using the default values
clone:
  skip: false
  depth: 1
  submodules: false

dependencies:
  # nixpkgs
  nixpkgs:
    - nodejs
    - go
  # custom registry
  git+https://tangled.org/@example.com/my_pkg:
    - my_pkg

environment:
  GOOS: "linux"
  GOARCH: "arm64"
  NODE_ENV: "production"
  MY_ENV_VAR: "MY_ENV_VALUE"

steps:
  - name: "Build backend"
    command: "go build"
    environment:
      GOOS: "darwin"
      GOARCH: "arm64"
  - name: "Build frontend"
    command: "npm run build"
    environment:
      NODE_ENV: "production"
```

If you want another example of a workflow, you can look at
the one [Tangled uses to build the
project](https://tangled.org/@tangled.org/core/blob/master/.tangled/workflows/build.yml).

## Self-hosting guide

### Prerequisites

- Go
- Docker (the only supported backend currently)

### Configuration

Spindle is configured using environment variables. The following environment variables are available:

- `SPINDLE_SERVER_LISTEN_ADDR`: The address the server listens on (default: `"0.0.0.0:6555"`).
- `SPINDLE_SERVER_DB_PATH`: The path to the SQLite database file (default: `"spindle.db"`).
- `SPINDLE_SERVER_HOSTNAME`: The hostname of the server (required).
- `SPINDLE_SERVER_JETSTREAM_ENDPOINT`: The endpoint of the Jetstream server (default: `"wss://jetstream1.us-west.bsky.network/subscribe"`).
- `SPINDLE_SERVER_DEV`: A boolean indicating whether the server is running in development mode (default: `false`).
- `SPINDLE_SERVER_OWNER`: The DID of the owner (required).
- `SPINDLE_PIPELINES_NIXERY`: The Nixery URL (default: `"nixery.tangled.sh"`).
- `SPINDLE_PIPELINES_WORKFLOW_TIMEOUT`: The default workflow timeout (default: `"5m"`).
- `SPINDLE_PIPELINES_LOG_DIR`: The directory to store workflow logs (default: `"/var/log/spindle"`).

### Running spindle

1.  **Set the environment variables.** For example:

    ```shell
    export SPINDLE_SERVER_HOSTNAME="your-hostname"
    export SPINDLE_SERVER_OWNER="your-did"
    ```

2.  **Build the Spindle binary.**

    ```shell
    cd core
    go mod download
    go build -o cmd/spindle/spindle cmd/spindle/main.go
    ```

3.  **Create the log directory.**

    ```shell
    sudo mkdir -p /var/log/spindle
    sudo chown $USER:$USER -R /var/log/spindle
    ```

4.  **Run the Spindle binary.**

    ```shell
    ./cmd/spindle/spindle
    ```

Spindle will now start, connect to the Jetstream server, and begin processing pipelines.

## Architecture

Spindle is a small CI runner service. Here's a high-level overview of how it operates:

- Listens for [`sh.tangled.spindle.member`](/lexicons/spindle/member.json) and
  [`sh.tangled.repo`](/lexicons/repo.json) records on the Jetstream.
- When a new repo record comes through (typically when you add a spindle to a
  repo from the settings), spindle then resolves the underlying knot and
  subscribes to repo events (see:
  [`sh.tangled.pipeline`](/lexicons/pipeline.json)).
- The spindle engine then handles execution of the pipeline, with results and
  logs beamed on the spindle event stream over WebSocket

### The engine

At present, the only supported backend is Docker (and Podman, if Docker
compatibility is enabled, so that `/run/docker.sock` is created). spindle
executes each step in the pipeline in a fresh container, with state persisted
across steps within the `/tangled/workspace` directory.

The base image for the container is constructed on the fly using
[Nixery](https://nixery.dev), which is handy for caching layers for frequently
used packages.

The pipeline manifest is [specified here](https://docs.tangled.org/spindles.html#pipelines).

## Secrets with openbao

This document covers setting up spindle to use OpenBao for secrets
management via OpenBao Proxy instead of the default SQLite backend.

### Overview

Spindle now uses OpenBao Proxy for secrets management. The proxy handles
authentication automatically using AppRole credentials, while spindle
connects to the local proxy instead of directly to the OpenBao server.

This approach provides better security, automatic token renewal, and
simplified application code.

### Installation

Install OpenBao from Nixpkgs:

```bash
nix shell nixpkgs#openbao   # for a local server
```

### Setup

The setup process can is documented for both local development and production.

#### Local development

Start OpenBao in dev mode:

```bash
bao server -dev -dev-root-token-id="root" -dev-listen-address=127.0.0.1:8201
```

This starts OpenBao on `http://localhost:8201` with a root token.

Set up environment for bao CLI:

```bash
export BAO_ADDR=http://localhost:8200
export BAO_TOKEN=root
```

#### Production

You would typically use a systemd service with a
configuration file. Refer to
[@tangled.org/infra](https://tangled.org/@tangled.org/infra)
for how this can be achieved using Nix.

Then, initialize the bao server:

```bash
bao operator init -key-shares=1 -key-threshold=1
```

This will print out an unseal key and a root key. Save them
somewhere (like a password manager). Then unseal the vault
to begin setting it up:

```bash
bao operator unseal <unseal_key>
```

All steps below remain the same across both dev and
production setups.

#### Configure openbao server

Create the spindle KV mount:

```bash
bao secrets enable -path=spindle -version=2 kv
```

Set up AppRole authentication and policy:

Create a policy file `spindle-policy.hcl`:

```hcl
# Full access to spindle KV v2 data
path "spindle/data/*" {
  capabilities = ["create", "read", "update", "delete"]
}

# Access to metadata for listing and management
path "spindle/metadata/*" {
  capabilities = ["list", "read", "delete", "update"]
}

# Allow listing at root level
path "spindle/" {
  capabilities = ["list"]
}

# Required for connection testing and health checks
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
```

Apply the policy and create an AppRole:

```bash
bao policy write spindle-policy spindle-policy.hcl
bao auth enable approle
bao write auth/approle/role/spindle \
    token_policies="spindle-policy" \
    token_ttl=1h \
    token_max_ttl=4h \
    bind_secret_id=true \
    secret_id_ttl=0 \
    secret_id_num_uses=0
```

Get the credentials:

```bash
# Get role ID (static)
ROLE_ID=$(bao read -field=role_id auth/approle/role/spindle/role-id)

# Generate secret ID
SECRET_ID=$(bao write -f -field=secret_id auth/approle/role/spindle/secret-id)

echo "Role ID: $ROLE_ID"
echo "Secret ID: $SECRET_ID"
```

#### Create proxy configuration

Create the credential files:

```bash
# Create directory for OpenBao files
mkdir -p /tmp/openbao

# Save credentials
echo "$ROLE_ID" > /tmp/openbao/role-id
echo "$SECRET_ID" > /tmp/openbao/secret-id
chmod 600 /tmp/openbao/role-id /tmp/openbao/secret-id
```

Create a proxy configuration file `/tmp/openbao/proxy.hcl`:

```hcl
# OpenBao server connection
vault {
  address = "http://localhost:8200"
}

# Auto-Auth using AppRole
auto_auth {
  method "approle" {
    mount_path = "auth/approle"
    config = {
      role_id_file_path   = "/tmp/openbao/role-id"
      secret_id_file_path = "/tmp/openbao/secret-id"
    }
  }

  # Optional: write token to file for debugging
  sink "file" {
    config = {
      path = "/tmp/openbao/token"
      mode = 0640
    }
  }
}

# Proxy listener for spindle
listener "tcp" {
  address     = "127.0.0.1:8201"
  tls_disable = true
}

# Enable API proxy with auto-auth token
api_proxy {
  use_auto_auth_token = true
}

# Enable response caching
cache {
  use_auto_auth_token = true
}

# Logging
log_level = "info"
```

#### Start the proxy

Start OpenBao Proxy:

```bash
bao proxy -config=/tmp/openbao/proxy.hcl
```

The proxy will authenticate with OpenBao and start listening on
`127.0.0.1:8201`.

#### Configure spindle

Set these environment variables for spindle:

```bash
export SPINDLE_SERVER_SECRETS_PROVIDER=openbao
export SPINDLE_SERVER_SECRETS_OPENBAO_PROXY_ADDR=http://127.0.0.1:8201
export SPINDLE_SERVER_SECRETS_OPENBAO_MOUNT=spindle
```

On startup, spindle will now connect to the local proxy,
which handles all authentication automatically.

### Production setup for proxy

For production, you'll want to run the proxy as a service:

Place your production configuration in
`/etc/openbao/proxy.hcl` with proper TLS settings for the
vault connection.

### Verifying setup

Test the proxy directly:

```bash
# Check proxy health
curl -H "X-Vault-Request: true" http://127.0.0.1:8201/v1/sys/health

# Test token lookup through proxy
curl -H "X-Vault-Request: true" http://127.0.0.1:8201/v1/auth/token/lookup-self
```

Test OpenBao operations through the server:

```bash
# List all secrets
bao kv list spindle/

# Add a test secret via the spindle API, then check it exists
bao kv list spindle/repos/

# Get a specific secret
bao kv get spindle/repos/your_repo_path/SECRET_NAME
```

### How it works

- Spindle connects to OpenBao Proxy on localhost (typically
  port 8200 or 8201)
- The proxy authenticates with OpenBao using AppRole
  credentials
- All spindle requests go through the proxy, which injects
  authentication tokens
- Secrets are stored at
  `spindle/repos/{sanitized_repo_path}/{secret_key}`
- Repository paths like `did:plc:alice/myrepo` become
  `did_plc_alice_myrepo`
- The proxy handles all token renewal automatically
- Spindle no longer manages tokens or authentication
  directly

### Troubleshooting

**Connection refused**: Check that the OpenBao Proxy is
running and listening on the configured address.

**403 errors**: Verify the AppRole credentials are correct
and the policy has the necessary permissions.

**404 route errors**: The spindle KV mount probably doesn't
exist—run the mount creation step again.

**Proxy authentication failures**: Check the proxy logs and
verify the role-id and secret-id files are readable and
contain valid credentials.

**Secret not found after writing**: This can indicate policy
permission issues. Verify the policy includes both
`spindle/data/*` and `spindle/metadata/*` paths with
appropriate capabilities.

Check proxy logs:

```bash
# If running as systemd service
journalctl -u openbao-proxy -f

# If running directly, check the console output
```

Test AppRole authentication manually:

```bash
bao write auth/approle/login \
    role_id="$(cat /tmp/openbao/role-id)" \
    secret_id="$(cat /tmp/openbao/secret-id)"
```

# Webhooks

Webhooks allow you to receive HTTP POST notifications when events occur in your repositories. This enables you to integrate Tangled with external services, trigger CI/CD pipelines, send notifications, or automate workflows.

## Overview

Webhooks send HTTP POST requests to URLs you configure whenever specific events happen. Currently, Tangled supports push events, with more event types coming soon.

## Configuring webhooks

To set up a webhook for your repository:

1. Navigate to your repository settings
2. Click the "hooks" tab
3. Click "add webhook"
4. Configure your webhook:
   - **Payload URL**: The endpoint that will receive the webhook POST requests
   - **Secret**: An optional secret key for verifying webhook authenticity (auto-generated if left blank)
   - **Events**: Select which events trigger the webhook (currently only push events)
   - **Active**: Toggle whether the webhook is enabled

## Webhook payload

### Push

When a push event occurs, Tangled sends a POST request with a JSON payload of the format:

```json
{
  "after": "7b320e5cbee2734071e4310c1d9ae401d8f6cab5",
  "before": "c04ddf64eddc90e4e2a9846ba3b43e67a0e2865e",
  "pusher": { 
    "did": "did:plc:hwevmowznbiukdf6uk5dwrrq" 
  },
  "ref": "refs/heads/main",
  "repository": {
    "clone_url": "https://tangled.org/did:plc:hwevmowznbiukdf6uk5dwrrq/some-repo",
    "created_at": "2025-09-15T08:57:23Z",
    "description": "an example repository",
    "fork": false,
    "full_name": "did:plc:hwevmowznbiukdf6uk5dwrrq/some-repo",
    "html_url": "https://tangled.org/did:plc:hwevmowznbiukdf6uk5dwrrq/some-repo",
    "name": "some-repo",
    "open_issues_count": 5,
    "owner": { 
      "did": "did:plc:hwevmowznbiukdf6uk5dwrrq" 
    },
    "ssh_url": "ssh://git@tangled.org/did:plc:hwevmowznbiukdf6uk5dwrrq/some-repo",
    "stars_count": 1,
    "updated_at": "2025-09-15T08:57:23Z"
  }
}
```

## HTTP headers

Each webhook request includes the following headers:

- `Content-Type: application/json`
- `User-Agent: Tangled-Hook/<short-sha>` — User agent with short SHA of the commit
- `X-Tangled-Event: push` — The event type
- `X-Tangled-Hook-ID: <webhook-id>` — The webhook ID
- `X-Tangled-Delivery: <uuid>` — Unique delivery ID
- `X-Tangled-Signature-256: sha256=<hmac>` — HMAC-SHA256 signature (if secret configured)

## Verifying webhook signatures

If you configured a secret, you should verify the webhook signature to ensure requests are authentic. For example, in Go:

```go
package main

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "io"
    "net/http"
    "strings"
)

func verifySignature(payload []byte, signatureHeader, secret string) bool {
    // Remove 'sha256=' prefix from signature header
    signature := strings.TrimPrefix(signatureHeader, "sha256=")
    
    // Compute expected signature
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    expected := hex.EncodeToString(mac.Sum(nil))
    
    // Use constant-time comparison to prevent timing attacks
    return hmac.Equal([]byte(signature), []byte(expected))
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
    // Read the request body
    payload, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Bad request", http.StatusBadRequest)
        return
    }
    
    // Get signature from header
    signatureHeader := r.Header.Get("X-Tangled-Signature-256")
    
    // Verify signature
    if signatureHeader != "" && verifySignature(payload, signatureHeader, yourSecret) {
        // Webhook is authentic, process it
        processWebhook(payload)
        w.WriteHeader(http.StatusOK)
    } else {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
    }
}
```

## Delivery retries

Webhooks are automatically retried on failure:

- **3 total attempts** (1 initial + 2 retries)
- **Exponential backoff** starting at 1 second, max 10 seconds
- **Retried on**:
  - Network errors
  - HTTP 5xx server errors
- **Not retried on**:
  - HTTP 4xx client errors (bad request, unauthorized, etc.)

### Timeouts

Webhook requests timeout after 30 seconds. If your endpoint needs more time:

1. Respond with 200 OK immediately
2. Process the webhook asynchronously in the background

## Example integrations

### Discord notifications

```javascript
app.post("/webhook", (req, res) => {
  const payload = req.body;

  fetch("https://discord.com/api/webhooks/...", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      content: `New push to ${payload.repository.full_name}`,
      embeds: [
        {
          title: `${payload.pusher.did} pushed to ${payload.ref}`,
          url: payload.repository.html_url,
          color: 0x00ff00,
        },
      ],
    }),
  });

  res.status(200).send("OK");
});
```

# Migrating knots and spindles

Sometimes, non-backwards compatible changes are made to the
knot/spindle XRPC APIs. If you host a knot or a spindle, you
will need to follow this guide to upgrade. Typically, this
only requires you to deploy the newest version.

This document is laid out in reverse-chronological order.
Newer migration guides are listed first, and older guides
are further down the page.

## Upgrading from v1.8.x

After v1.8.2, the HTTP API for knots and spindles has been
deprecated and replaced with XRPC. Repositories on outdated
knots will not be viewable from the appview. Upgrading is
straightforward however.

For knots:

- Upgrade to the latest tag (v1.9.0 or above)
- Head to the [knot dashboard](https://tangled.org/settings/knots) and
  hit the "retry" button to verify your knot

For spindles:

- Upgrade to the latest tag (v1.9.0 or above)
- Head to the [spindle
  dashboard](https://tangled.org/settings/spindles) and hit the
  "retry" button to verify your spindle

## Upgrading from v1.7.x

After v1.7.0, knot secrets have been deprecated. You no
longer need a secret from the appview to run a knot. All
authorized commands to knots are managed via [Inter-Service
Authentication](https://atproto.com/specs/xrpc#inter-service-authentication-jwt).
Knots will be read-only until upgraded.

Upgrading is quite easy, in essence:

- `KNOT_SERVER_SECRET` is no more, you can remove this
  environment variable entirely
- `KNOT_SERVER_OWNER` is now required on boot, set this to
  your DID. You can find your DID in the
  [settings](https://tangled.org/settings) page.
- Restart your knot once you have replaced the environment
  variable
- Head to the [knot dashboard](https://tangled.org/settings/knots) and
  hit the "retry" button to verify your knot. This simply
  writes a `sh.tangled.knot` record to your PDS.

If you use the nix module, simply bump the flake to the
latest revision, and change your config block like so:

```diff
 services.tangled.knot = {
   enable = true;
   server = {
-    secretFile = /path/to/secret;
+    owner = "did:plc:foo";
   };
 };
```

# Hacking on Tangled

We highly recommend [installing
Nix](https://nixos.org/download/) (the package manager)
before working on the codebase. The Nix flake provides a lot
of helpers to get started and most importantly, builds and
dev shells are entirely deterministic.

To set up your dev environment:

```bash
nix develop
```

Non-Nix users can look at the `devShell` attribute in the
`flake.nix` file to determine necessary dependencies.

## Running the appview

The Nix flake also exposes a few `app` attributes (run `nix
flake show` to see a full list of what the flake provides),
one of the apps runs the appview with the `air`
live-reloader:

```bash
TANGLED_DEV=true nix run .#watch-appview

# TANGLED_DB_PATH might be of interest to point to
# different sqlite DBs

# in a separate shell, you can live-reload tailwind
nix run .#watch-tailwind
```

To authenticate with the appview, you will need Redis and
OAuth JWKs to be set up:

```
# OAuth JWKs should already be set up by the Nix devshell:
echo $TANGLED_OAUTH_CLIENT_SECRET
z42ty4RT1ovnTopY8B8ekz9NuziF2CuMkZ7rbRFpAR9jBqMc

echo $TANGLED_OAUTH_CLIENT_KID
1761667908

# if not, you can set it up yourself:
goat key generate -t P-256
Key Type: P-256 / secp256r1 / ES256 private key
Secret Key (Multibase Syntax): save this securely (eg, add to password manager)
        z42tuPDKRfM2mz2Kv953ARen2jmrPA8S9LX9tRq4RVcUMwwL
Public Key (DID Key Syntax): share or publish this (eg, in DID document)
        did:key:zDnaeUBxtG6Xuv3ATJE4GaWeyXM3jyamJsZw3bSPpxx4bNXDR

# the secret key from above
export TANGLED_OAUTH_CLIENT_SECRET="z42tuP..."

# Run Redis in a new shell to store OAuth sessions
redis-server
```

## Running knots and spindles

An end-to-end knot setup requires setting up a machine with
`sshd`, `AuthorizedKeysCommand`, and a Git user, which is
quite cumbersome. So the Nix flake provides a
`nixosConfiguration` to do so.

<details>
  <summary><strong>macOS users will have to set up a Nix Builder first</strong></summary>

In order to build Tangled's dev VM on macOS, you will
first need to set up a Linux Nix builder. The recommended
way to do so is to run a [`darwin.linux-builder`
VM](https://nixos.org/manual/nixpkgs/unstable/#sec-darwin-builder)
and to register it in `nix.conf` as a builder for Linux
with the same architecture as your Mac (`linux-aarch64` if
you are using Apple Silicon).

> IMPORTANT: You must build `darwin.linux-builder` somewhere other than inside
> the Tangled repo so that it doesn't conflict with the other VM. For example,
> you can do
>
> ```shell
> cd $(mktemp -d buildervm.XXXXX) && nix run nixpkgs#darwin.linux-builder
> ```
>
> to store the builder VM in a temporary dir.
>
> You should read and follow [all the other intructions][darwin builder vm] to
> avoid subtle problems.

Alternatively, you can use any other method to set up a
Linux machine with Nix installed that you can `sudo ssh`
into (in other words, root user on your Mac has to be able
to ssh into the Linux machine without entering a password)
and that has the same architecture as your Mac. See
[remote builder
instructions](https://nix.dev/manual/nix/2.28/advanced-topics/distributed-builds.html#requirements)
for how to register such a builder in `nix.conf`.

> WARNING: If you'd like to use
> [`nixos-lima`](https://github.com/nixos-lima/nixos-lima) or
> [Orbstack](https://orbstack.dev/), note that setting them up so that `sudo
ssh` works can be tricky. It seems to be [possible with
> Orbstack](https://github.com/orgs/orbstack/discussions/1669).

</details>

To begin, grab your DID from http://localhost:3000/settings.
Then, set `TANGLED_VM_KNOT_OWNER` and
`TANGLED_VM_SPINDLE_OWNER` to your DID. You can now start a
lightweight NixOS VM like so:

```bash
nix run --impure .#vm

# type `poweroff` at the shell to exit the VM
```

This starts a knot on port 6444, a spindle on port 6555
with `ssh` exposed on port 2222.

Once the services are running, head to
http://localhost:3000/settings/knots and hit "Verify". It should
verify the ownership of the services instantly if everything
went smoothly.

You can push repositories to this VM with this ssh config
block on your main machine:

```bash
Host nixos-shell
    Hostname localhost
    Port 2222
    User git
    IdentityFile ~/.ssh/my_tangled_key
```

Set up a remote called `local-dev` on a git repo:

```bash
git remote add local-dev git@nixos-shell:user/repo
git push local-dev main
```

The above VM should already be running a spindle on
`localhost:6555`. Head to http://localhost:3000/settings/spindles and
hit "Verify". You can then configure each repository to use
this spindle and run CI jobs.

Of interest when debugging spindles:

```
# Service logs from journald:
journalctl -xeu spindle

# CI job logs from disk:
ls /var/log/spindle

# Debugging spindle database:
sqlite3 /var/lib/spindle/spindle.db

# litecli has a nicer REPL interface:
litecli /var/lib/spindle/spindle.db
```

If for any reason you wish to disable either one of the
services in the VM, modify [nix/vm.nix](/nix/vm.nix) and set
`services.tangled.spindle.enable` (or
`services.tangled.knot.enable`) to `false`.

# Contribution guide

## Commit guidelines

We follow a commit style similar to the Go project. Please keep commits:

- **atomic**: each commit should represent one logical change
- **descriptive**: the commit message should clearly describe what the
  change does and why it's needed

### Message format

```
<service/top-level directory>/<affected package/directory>: <short summary of change>

Optional longer description can go here, if necessary. Explain what the
change does and why, especially if not obvious. Reference relevant
issues or PRs when applicable. These can be links for now since we don't
auto-link issues/PRs yet.
```

Here are some examples:

```
appview/state: fix token expiry check in middleware

The previous check did not account for clock drift, leading to premature
token invalidation.
```

```
knotserver/git/service: improve error checking in upload-pack
```

### General notes

- PRs get merged "as-is" (fast-forward)—like applying a patch-series
  using `git am`. At present, there is no squashing—so please author
  your commits as they would appear on `master`, following the above
  guidelines.
- If there is a lot of nesting, for example "appview:
  pages/templates/repo/fragments: ...", these can be truncated down to
  just "appview: repo/fragments: ...". If the change affects a lot of
  subdirectories, you may abbreviate to just the top-level names, e.g.
  "appview: ..." or "knotserver: ...".
- Keep commits lowercased with no trailing period.
- Use the imperative mood in the summary line (e.g., "fix bug" not
  "fixed bug" or "fixes bug").
- Try to keep the summary line under 72 characters, but we aren't too
  fussed about this.
- Follow the same formatting for PR titles if filled manually.
- Don't include unrelated changes in the same commit.
- Avoid noisy commit messages like "wip" or "final fix"—rewrite history
  before submitting if necessary.

## Code formatting

We use a variety of tools to format our code, and multiplex them with
[`treefmt`](https://treefmt.com). All you need to do to format your changes
is run `nix run .#fmt` (or just `treefmt` if you're in the devshell).

## Proposals for bigger changes

Small fixes like typos, minor bugs, or trivial refactors can be
submitted directly as PRs.

For larger changes—especially those introducing new features, significant
refactoring, or altering system behavior—please open a proposal first. This
helps us evaluate the scope, design, and potential impact before implementation.

Create a new issue titled:

```
proposal: <affected scope>: <summary of change>
```

In the description, explain:

- What the change is
- Why it's needed
- How you plan to implement it (roughly)
- Any open questions or tradeoffs

We'll use the issue thread to discuss and refine the idea before moving
forward.

## Developer Certificate of Origin (DCO)

We require all contributors to certify that they have the right to
submit the code they're contributing. To do this, we follow the
[Developer Certificate of Origin
(DCO)](https://developercertificate.org/).

By signing your commits, you're stating that the contribution is your
own work, or that you have the right to submit it under the project's
license. This helps us keep things clean and legally sound.

To sign your commit, just add the `-s` flag when committing:

```sh
git commit -s -m "your commit message"
```

This appends a line like:

```
Signed-off-by: Your Name <your.email@example.com>
```

We won't merge commits if they aren't signed off. If you forget, you can
amend the last commit like this:

```sh
git commit --amend -s
```

If you're submitting a PR with multiple commits, make sure each one is
signed.

For [jj](https://jj-vcs.github.io/jj/latest/) users, you can run the following command
to make it sign off commits in the tangled repo:

```shell
# Safety check, should say "No matching config key..."
jj config list templates.commit_trailers
# The command below may need to be adjusted if the command above returned something.
jj config set --repo templates.commit_trailers "format_signed_off_by_trailer(self)"
```

Refer to the [jujutsu
documentation](https://jj-vcs.github.io/jj/latest/config/#commit-trailers)
for more information.

# Troubleshooting guide

## Login issues

Owing to the distributed nature of OAuth on AT Protocol, you
may run into issues with logging in. If you run a
self-hosted PDS:

- You may need to ensure that your PDS is timesynced using
  NTP:
  - Enable the `ntpd` service
  - Run `ntpd -qg` to synchronize your clock
- You may need to increase the default request timeout:
  `NODE_OPTIONS="--network-family-autoselection-attempt-timeout=500"`

## Empty punchcard

For Tangled to register commits that you make across the
network, you need to setup one of following:

- The committer email should be a verified email associated
  to your account. You can add and verify emails on the
  settings page.
- Or, the committer email should be set to your account's
  DID: `git config user.email "did:plc:foobar"`. You can find
  your account's DID on the settings page

## Commit is not marked as verified

Presently, Tangled only supports SSH commit signatures.

To sign commits using an SSH key with git:

```
git config --global gpg.format ssh
git config --global user.signingkey ~/.ssh/tangled-key
```

To sign commits using an SSH key with jj, add this to your
config:

```
[signing]
behavior = "own"
backend = "ssh"
key = "~/.ssh/tangled-key"
```

## Self-hosted knot issues

If you need help troubleshooting a self-hosted knot, check
out the [knot troubleshooting
guide](/knot-self-hosting-guide.html#troubleshooting).
