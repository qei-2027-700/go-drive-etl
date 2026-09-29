# Data Portal dashboard (formerly Looker Studio)

This project connects Data Portal (formerly Looker Studio) directly to the two
Gold Mart tables. The dashboard definition lives in Data Portal, but its inputs, access identity,
and chart specification are versioned here so it can be rebuilt consistently.

## Access model

Terraform creates `looker-studio-bi` and grants it only:

| Resource | Role | Purpose |
| --- | --- | --- |
| GCP project | `roles/bigquery.jobUser` | Create query jobs in the billing project |
| `etl_gold` dataset | `roles/bigquery.dataViewer` | Read the two Gold Mart tables |
| service account itself | `roles/iam.serviceAccountTokenCreator` to the Data Portal service agent | Let Data Portal use the identity |

It receives no access to the Bronze (`etl_raw`) or Silver (`etl_silver`)
datasets and no write role. `looker_studio_data_source_editor_members` limits
which people or groups can choose the identity in a data source.

Service-account credentials require a Google Workspace or Cloud Identity
managed organization. With such an account, obtain the organization service
agent principal (`service-org-...@gcp-sa-datastudio.iam.gserviceaccount.com`),
set it as `looker_studio_service_agent_member`, and set the editor principals.
Then run:

```bash
cd iac
terraform init
terraform apply
```

Do not create or download a JSON key for this account. Data Portal obtains
short-lived credentials through its service agent. For a personal Google
account, leave both `looker_studio_*` variables unset: Terraform still creates
the isolated, read-only BI identity, but the report must use **Owner's
credentials** instead. This is the supported path when no organization service
agent is available.

## Create the data sources

Refresh the Gold marts first so the tables exist and hold current data:

```bash
BQ_PROJECT_ID=your-gcp-project-id ./scripts/refresh_gold_marts.sh
```

In Data Portal, create a report and add the **BigQuery** connector twice. If
using a managed organization, select **Service account credentials**, enter the
Terraform output `looker_studio_service_account_email`, and use the same
billing project as `project_id`. With a personal Google account, retain the
default **Owner's credentials** and ensure that the report owner has
`bigquery.dataViewer` on `etl_gold` and `bigquery.jobUser` on the billing
project.

| Data source name | Table | Use |
| --- | --- | --- |
| `Gold ingestion daily` | `etl_gold.mart_ingestion_daily` | Daily trend and embedding status |
| `Gold file latest` | `etl_gold.mart_file_latest` | Latest file and chunk totals |

Use table sources rather than custom SQL. All aggregation logic is retained in
the versioned Gold SQL, and the BI identity needs access only to `etl_gold`.

## Dashboard: Ingestion health

Build one page titled **Ingestion health** using the following charts. The
layout makes volume, freshness, and embedding backlog visible without exposing
chunk contents.

| Position | Chart | Source and configuration |
| --- | --- | --- |
| Top | Scorecard: files ingested | `Gold ingestion daily`; metric `SUM(file_count)` |
| Top | Scorecard: pending embeddings | `Gold ingestion daily`; metric `SUM(pending_embedding_chunk_count)` |
| Middle | Time series: daily ingestion | `Gold ingestion daily`; dimension `ingested_date`; metric `chunk_count` |
| Bottom | Table: latest file summary | `Gold file latest`; dimensions `file_id`, `latest_ingested_at`; metric `Record Count` |

Add a report-level date-range control for `ingested_date`. Set the report to
use the service-account credentials, and share the report link only after
verifying that a non-editor viewer can load the charts without BigQuery access.

## Acceptance check

1. `terraform apply` creates the identity and all non-empty IAM bindings.
2. In Data Portal, each source passes its connection test and the dashboard
   renders after a Gold refresh.
3. Verify the service account cannot read `etl_raw` or `etl_silver` with a
   BigQuery query; this confirms that the connection is bounded to Gold.
4. Capture the resulting report page and add it to the README before publishing
   the repository. This last step requires the project's Data Portal account
   and cannot be generated from Terraform.
