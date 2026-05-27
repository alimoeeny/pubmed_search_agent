# PubMed Agent — Deployment Guide

Step-by-step guide to deploy the full PubMed Agent stack from scratch: Supabase auth + DB, GCP Cloud Run backend, and Cloudflare Pages frontend.

---

## Prerequisites

Before starting, make sure you have:

- [ ] `gcloud` CLI installed and authenticated (`gcloud auth login`)
- [ ] `terraform` >= 1.6 installed (`terraform -version`)
- [ ] GCP account with a billing account
- [ ] [Supabase](https://supabase.com) account (free tier works)
- [ ] [Cloudflare](https://cloudflare.com) account with `ai-goblins.com` already managed there
- [ ] The repo cloned locally with all changes committed to `main`

---

## Step 1 — Set up Supabase

Supabase provides auth (JWT) and the Postgres session store. Both backend and frontend depend on it.

1. Create a new project at [supabase.com](https://supabase.com) (free tier; pick a region close to `us-east1`)
2. Wait ~2 minutes for the database to provision
3. Apply the two migrations — use the **SQL Editor** tab in the dashboard, or via `psql`:

```bash
SUPABASE_DB_URL="postgresql://postgres:<password>@db.<ref>.supabase.co:5432/postgres"

psql "$SUPABASE_DB_URL" -f db/migrations/001_create_user_profiles.sql
psql "$SUPABASE_DB_URL" -f db/migrations/002_create_sessions.sql
```

4. Collect these values for later steps:

| Value | Where to find it |
|-------|------------------|
| **JWT Secret** | Project Settings → API → JWT Secret |
| **DB URL** | Project Settings → Database → Connection string → URI mode |
| **Supabase URL** | Project Settings → API → Project URL |
| **Anon Key** | Project Settings → API → Project API keys → `anon public` |

> The Supabase URL and Anon Key are already hardcoded in `www/src/lib/config.ts`. You only need to update those constants if you created a brand new Supabase project.

5. In the Supabase dashboard → **Authentication → URL Configuration**:
   - **Site URL**: `https://pubmed.ai-goblins.com`
   - **Redirect URLs**: add `https://pubmed.ai-goblins.com/auth/callback`

---

## Step 2 — Create a GCP project

```bash
gcloud projects create <project-id> --name="PubMed Agent"
gcloud config set project <project-id>

# Link billing (required for Cloud Run, Artifact Registry, Secret Manager)
gcloud billing projects link <project-id> --billing-account=<billing-account-id>
```

Find your billing account ID:
```bash
gcloud billing accounts list
```

---

## Step 3 — Enable required GCP APIs

```bash
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  cloudresourcemanager.googleapis.com \
  storage.googleapis.com
```

Takes ~1 minute. One-time per project.

---

## Step 4 — Configure and apply Terraform

```bash
cd infra/envs/prod
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
project_id           = "<your-gcp-project-id>"
github_repo          = "<owner>/<repo>"          # e.g. "alimoeeny/pubmed_search"
image_tag            = "latest"
supabase_url         = "https://<ref>.supabase.co"
cors_allowed_origins = "https://pubmed.ai-goblins.com"

secrets = {
  SUPABASE_DB_URL = "<db-url-from-step-1>"
}
```

> **No API key required.** Terraform grants the Cloud Run service account `roles/aiplatform.user`, so the service calls Vertex AI (Gemini) using its own IAM identity — no `GOOGLE_API_KEY` secret needed. NCBI email is drawn from each authenticated user's Supabase profile at request time.

> `terraform.tfvars` is git-ignored. Never commit it.

```bash
terraform init
terraform plan     # review 15+ resources before applying
terraform apply    # type "yes" when prompted
```

Apply takes 2–3 minutes (16 resources). **Note the four outputs** (or retrieve later with `terraform output`):

```
workload_identity_provider = "projects/.../providers/github-provider"
github_actions_sa_email    = "pubmed-agent-deploy@<project>.iam.gserviceaccount.com"
artifact_registry_url      = "us-east1-docker.pkg.dev/<project>/pubmed-agent"
cloud_run_url              = "https://pubmed-agent-xxxx-ue.a.run.app"
```

---

## Step 5 — Configure GitHub repository secrets

In **Settings → Secrets and variables → Actions**, add three **repository secrets**:

| Secret | Value |
|--------|-------|
| `GCP_PROJECT_ID` | Your GCP project ID |
| `WIF_PROVIDER` | `workload_identity_provider` output from Step 4 |
| `WIF_SERVICE_ACCOUNT` | `github_actions_sa_email` output from Step 4 |

In **Settings → Environments**, create an environment named exactly **`production`**.

> The workflow (`.github/workflows/deploy-prod.yml`) requires the `production` environment before the deploy job will run.

---

## Step 6 — First backend deploy

```bash
git push origin main
```

GitHub Actions runs two jobs:
1. **test** — `go test ./...`; deploy is blocked if any test fails
2. **deploy** — Docker build + push to Artifact Registry + Cloud Run deploy

Watch at `https://github.com/<owner>/<repo>/actions`. First build: ~4 min. Subsequent: ~90 sec.

**Verify:**
```bash
CLOUD_RUN_URL=$(terraform -chdir=infra/envs/prod output -raw cloud_run_url)
curl $CLOUD_RUN_URL/healthz   # expect HTTP 200
```

---

## Step 7 — Map custom backend domain (`api.pubmedagent.ai-goblins.com`)

**7a. Create the Cloud Run domain mapping:**
```bash
gcloud run domain-mappings create \
  --service pubmed-agent \
  --domain api.pubmedagent.ai-goblins.com \
  --region us-east1
```

**7b. Get the required DNS records:**
```bash
gcloud run domain-mappings describe \
  --domain api.pubmedagent.ai-goblins.com \
  --region us-east1
```
The output lists one or more DNS records (typically a CNAME to `ghs.googlehosted.com`).

**7c. Add the record in Cloudflare DNS:**
- Dashboard → ai-goblins.com → DNS → Add record
- Type: `CNAME`, Name: `api.pubmedagent`, Target: `ghs.googlehosted.com`
- **Proxy: OFF** (grey cloud — DNS only). Cloud Run manages TLS directly; Cloudflare proxy causes certificate conflicts.

**7d. Verify (~5 min for DNS + Google cert provisioning):**
```bash
curl https://api.pubmedagent.ai-goblins.com/healthz   # expect HTTP 200
```

---

## Step 8 — Deploy frontend via Cloudflare Pages

**8a. Connect the repo:**
Cloudflare Dashboard → **Pages** → Create a project → Connect to Git → select this repo

**8b. Configure the build:**

| Setting | Value |
|---------|-------|
| Framework preset | None |
| Root directory | `www` |
| Build command | `pnpm run build:prod` |
| Build output directory | `dist` |
| Environment variable | `NODE_VERSION` = `20` |

**8c. Save and Deploy.**
First deploy: ~2 minutes. App goes live at `https://<project-name>.pages.dev`.

From this point, every push to `main` triggers an automatic Pages rebuild.

---

## Step 9 — Map custom frontend domain (`pubmed.ai-goblins.com`)

- Cloudflare Pages → your project → **Custom domains** → Add a custom domain
- Enter `pubmed.ai-goblins.com`
- Since the domain is already managed by Cloudflare, the CNAME and TLS certificate are provisioned **automatically** — no manual DNS step needed
- Active within ~1 minute

---

## Step 10 — End-to-end verification

```bash
# Backend health
curl https://api.pubmedagent.ai-goblins.com/healthz

# Create a session (requires a Supabase access_token)
curl -X POST https://api.pubmedagent.ai-goblins.com/v1/sessions \
  -H "Authorization: Bearer <supabase-access-token>"
# Expected: {"session_id":"..."}
```

Open `https://pubmed.ai-goblins.com` in a browser:
1. Sign in via Supabase auth
2. Create a session and send a message
3. Confirm SSE events arrive and text streams correctly

**CORS check:** DevTools → Network → any `/v1/` request. The response must include:
```
Access-Control-Allow-Origin: https://pubmed.ai-goblins.com
```
If you see `*` or a missing header, verify `cors_allowed_origins` in `terraform.tfvars` matches exactly (no trailing slash) and re-apply Terraform.

---

## Subsequent deploys

| What changed | How to deploy |
|---|---|
| Backend Go code | Push to `main` → GitHub Actions deploys automatically |
| Frontend code | Push to `main` → Cloudflare Pages deploys automatically |
| Terraform config | `cd infra/envs/prod && terraform apply` |
| Secrets | Update value in `terraform.tfvars` → `terraform apply` |

---

## Troubleshooting

**`terraform apply` fails with "API not enabled"**
Re-run Step 3. Propagation can take a few minutes.

**GitHub Actions "Permission denied" on Artifact Registry push**
Verify `WIF_PROVIDER` and `WIF_SERVICE_ACCOUNT` match Terraform outputs exactly (full resource path including `projects/...`).

**Cloud Run returns 500**
Check logs:
```bash
gcloud run services logs read pubmed-agent --region=us-east1 --limit=50
```
Most common cause: missing or incorrect Secret Manager value.

**`api.pubmedagent.ai-goblins.com` returns SSL error**
Ensure Cloudflare proxy is **OFF** (grey cloud) for that DNS record. Cloud Run manages its own TLS; an orange-cloud proxy creates a certificate conflict.

**Supabase connection refused from Cloud Run**
Use the **direct connection URI** (not the pooler URL). Project Settings → Database → Connection string → URI mode (not "Connection pooling").

**Supabase auth redirect fails after sign-in**
Confirm `https://pubmed.ai-goblins.com/auth/callback` is in the Supabase **Redirect URLs** list (Step 1).

**Browser CORS error on API calls**
`cors_allowed_origins` in `terraform.tfvars` must equal `https://pubmed.ai-goblins.com` with no trailing slash. After changing: `terraform apply` then redeploy backend (`git push origin main`).

**Cloudflare Pages build fails — pnpm not found**
Add `NODE_VERSION = 20` as a Pages environment variable. Cloudflare auto-detects the package manager from `pnpm-lock.yaml` when the Node version is set explicitly.
