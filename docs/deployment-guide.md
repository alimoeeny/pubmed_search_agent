# PubMed Agent — Deployment Guide

Step-by-step guide to deploy the PubMed Agent to GCP Cloud Run from a brand new GCP project.

---

## Prerequisites

Before starting, make sure you have:

- [ ] `gcloud` CLI installed and authenticated (`gcloud auth login`)
- [ ] `terraform` >= 1.6 installed (`terraform -version`)
- [ ] A GCP billing account (free trial works)
- [ ] A [Supabase](https://supabase.com) account (free tier works)
- [ ] The repo cloned locally with all changes committed to `main`

---

## Step 1 — Create a GCP project

```bash
gcloud projects create <project-id> --name="PubMed Agent"
gcloud config set project <project-id>

# Link billing (required for Cloud Run, Artifact Registry, Secret Manager)
gcloud billing projects link <project-id> --billing-account=<billing-account-id>
```

To find your billing account ID:
```bash
gcloud billing accounts list
```

---

## Step 2 — Enable required GCP APIs

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

This takes ~1 minute. You only need to do this once per project.

---

## Step 3 — Set up Supabase

1. Create a new project at [supabase.com](https://supabase.com)
2. Wait for the database to finish provisioning (~2 minutes)
3. Apply the two migrations. You can run these in the **SQL Editor** tab in the Supabase dashboard, or via `psql`:

```bash
SUPABASE_DB_URL="postgresql://postgres:<password>@db.<ref>.supabase.co:5432/postgres"

psql "$SUPABASE_DB_URL" -f db/migrations/001_create_user_profiles.sql
psql "$SUPABASE_DB_URL" -f db/migrations/002_create_sessions.sql
```

4. Copy these two values — you'll need them for Terraform:

| Value | Where to find it |
|-------|-----------------|
| **JWT Secret** | Project Settings → API → JWT Secret |
| **DB URL** | Project Settings → Database → Connection string → URI mode |

---

## Step 4 — Configure and apply Terraform

```bash
cd infra/envs/prod

cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` and fill in all values:

```hcl
project_id  = "<your-gcp-project-id>"
github_repo = "<github-username>/<repo-name>"   # e.g. "alimoeeny/pubmed_search"
image_tag   = "latest"

# Your Cloudflare Pages URL(s) — the browser blocks credentialed (JWT) requests
# unless the API echoes back the exact origin. Comma-separate multiple URLs.
cors_allowed_origins = "https://your-app.pages.dev,https://app.pubmedagent.ai-goblins.com"

secrets = {
  GOOGLE_API_KEY      = "<your-gemini-api-key>"
  NCBI_EMAIL          = "you@example.com"
  SUPABASE_JWT_SECRET = "<jwt-secret-from-step-3>"
  SUPABASE_DB_URL     = "<db-url-from-step-3>"
}
```

> `terraform.tfvars` is git-ignored. Never commit it.

Now initialise and apply:

```bash
terraform init
terraform plan    # review the 15 resources that will be created
terraform apply   # type "yes" when prompted
```

Apply takes 2–3 minutes. When it finishes, **note the four outputs**:

```
workload_identity_provider = "projects/.../providers/github-provider"
github_actions_sa_email    = "pubmed-agent-deploy@<project>.iam.gserviceaccount.com"
artifact_registry_url      = "us-east1-docker.pkg.dev/<project>/pubmed-agent"
cloud_run_url              = "https://pubmed-agent-xxxx-ue.a.run.app"
```

You can retrieve them again at any time with:
```bash
terraform output
```

---

## Step 5 — Configure GitHub repository secrets

In your GitHub repo go to **Settings → Secrets and variables → Actions** and add these three **repository secrets**:

| Secret name | Value |
|-------------|-------|
| `GCP_PROJECT_ID` | Your GCP project ID |
| `WIF_PROVIDER` | `workload_identity_provider` output from Step 4 |
| `WIF_SERVICE_ACCOUNT` | `github_actions_sa_email` output from Step 4 |

Then go to **Settings → Environments** and create an environment named exactly **`production`**.

> The workflow file (`.github/workflows/deploy-prod.yml`) requires the `production` environment to exist before it will run the deploy job.

---

## Step 6 — First deploy

Push to `main` to trigger the pipeline:

```bash
git push origin main
```

The GitHub Actions workflow runs two jobs in sequence:

1. **test** — runs `go test ./...`; deploy is blocked if any test fails
2. **deploy** — builds the Docker image, pushes to Artifact Registry, deploys to Cloud Run

Watch progress at:
```
https://github.com/<owner>/<repo>/actions
```

The first build takes ~3–4 minutes (Go module download + Docker layer cache is cold). Subsequent deploys are ~90 seconds.

---

## Step 7 — Verify the deployment

```bash
CLOUD_RUN_URL=$(terraform -chdir=infra/envs/prod output -raw cloud_run_url)

# Health check
curl $CLOUD_RUN_URL/healthz
# Expected: HTTP 200

# Create a session (JWT required in prod — use your Supabase access_token)
curl -X POST $CLOUD_RUN_URL/v1/sessions \
  -H "Authorization: Bearer <supabase-access-token>"
# Expected: {"session_id":"..."}
```

> **CORS check:** open your Cloudflare Pages URL in the browser and open DevTools → Network. If you see `Access-Control-Allow-Origin` matching your Pages URL on API responses, CORS is configured correctly. If you see `*` or a missing header, double-check `cors_allowed_origins` in `terraform.tfvars` and re-apply.

---

## Subsequent deploys

Every push to `main` triggers the full pipeline automatically. No manual steps needed.

The pipeline always:
1. Runs tests first — a failing test prevents deploy
2. Builds a new image tagged with the git SHA
3. Deploys the new revision to Cloud Run with zero downtime

---

## Troubleshooting

**`terraform apply` fails with "API not enabled"**
Re-run Step 2. GCP API enablement can take a few minutes to propagate.

**GitHub Actions "Permission denied" on Artifact Registry push**
Verify `WIF_PROVIDER` and `WIF_SERVICE_ACCOUNT` secrets match the Terraform outputs exactly, including the full resource path.

**Cloud Run returns 500 on every request**
Check Cloud Run logs:
```bash
gcloud run services logs read pubmed-agent --region=us-east1 --limit=50
```
Most common cause: a missing or incorrect secret in Secret Manager.

**Supabase connection refused from Cloud Run**
Ensure `SUPABASE_DB_URL` uses the **direct connection URI** (not the pooler URL). The pooler requires different TLS settings.

**Browser blocks API calls with "CORS error" or "Network Error" from the frontend**
The API must echo back the exact requesting origin (not `*`) for credentialed requests (`Authorization` header). Ensure `cors_allowed_origins` in `terraform.tfvars` contains your Cloudflare Pages URL exactly (no trailing slash). After updating, re-run `terraform apply` and redeploy.

For local development (`SERVER=true`), leave `CORS_ALLOWED_ORIGINS` unset — the middleware will reflect any origin back automatically.
