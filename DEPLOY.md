# Vici Dialer CDR Scrubber - Google Cloud Run Deployment Guide

## Prerequisites

1. **Google Cloud Account**: https://console.cloud.google.com/
2. **gcloud CLI**: https://cloud.google.com/sdk/docs/install
3. **Docker**: https://docs.docker.com/get-docker/

## Quick Deploy Steps

### 1. Install gcloud CLI
```bash
# Windows (PowerShell)
(New-Object Net.WebClient).DownloadFile("https://dl.google.com/dl/cloudsdk/channels/rapid/GoogleCloudSDKInstaller.exe", "$env:Temp\gcloud_installer.exe"); & "$env:Temp\gcloud_installer.exe"
```

### 2. Authenticate gcloud
```bash
gcloud auth login
gcloud auth configure-docker
```

### 3. Set your project
```bash
gcloud config set project YOUR_PROJECT_ID
```

### 4. Enable required APIs
```bash
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com
gcloud services enable cloudbuild.googleapis.com
```

### 5. Build and deploy
```bash
# Build the Docker image
docker build -t gcr.io/YOUR_PROJECT_ID/vici-cdr-scrubber .

# Push to Container Registry
docker push gcr.io/YOUR_PROJECT_ID/vici-cdr-scrubber

# Deploy to Cloud Run
gcloud run deploy vici-cdr-scrubber \
  --image gcr.io/YOUR_PROJECT_ID/vici-cdr-scrubber \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --memory 2Gi \
  --cpu 2 \
  --timeout 3600 \
  --set-env-vars="CONFIG_PATH=/root/config.yaml"
```

### 6. Set up environment variables (for database)
```bash
gcloud run services update vici-cdr-scrubber \
  --update-env-vars="DB_HOST=your-db-host,DB_PORT=5432,DB_USER=your-db-user,DB_PASSWORD=your-db-password,DB_NAME=vicidial"
```

## API Endpoints

After deployment, your service will be available at:
`https://vici-cdr-scrubber-xxxx-uc.a.run.app`

- `GET /` - API documentation
- `GET /health` - Health check
- `GET /status` - System status
- `GET /version` - Version info
- `POST /scrub` - Scrub CDR data
- `POST /profile` - Profile data quality
- `POST /validate` - Validate CDR records

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `CONFIG_PATH` | Path to config file | `/root/config.yaml` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `vicidial` |
| `DB_PASSWORD` | PostgreSQL password | - |
| `DB_NAME` | PostgreSQL database | `vicidial` |

## Cost Estimation

Google Cloud Run offers:
- **Free tier**: 240,000 vCPU-seconds/month
- **Free tier**: 450,000 GiB-seconds/month
- **Free tier**: 2 million requests/month

For a small deployment, you may stay within the free tier.
