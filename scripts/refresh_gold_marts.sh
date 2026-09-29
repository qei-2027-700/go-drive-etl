#!/usr/bin/env bash
set -euo pipefail

project_id="${BQ_PROJECT_ID:?Set BQ_PROJECT_ID to the GCP project ID}"
silver_dataset_id="${BQ_SILVER_DATASET_ID:-etl_silver}"
gold_dataset_id="${BQ_GOLD_DATASET_ID:-etl_gold}"
location="${BQ_LOCATION:-asia-northeast1}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

bq query \
  --use_legacy_sql=false \
  --project_id="${project_id}" \
  --location="${location}" \
  --parameter="project_id::${project_id}" \
  --parameter="silver_dataset_id::${silver_dataset_id}" \
  --parameter="gold_dataset_id::${gold_dataset_id}" \
  < "${script_dir}/../sql/refresh_gold_marts.sql"
