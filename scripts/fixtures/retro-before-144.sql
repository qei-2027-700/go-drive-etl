-- Baseline queries from fc42119 (PR #143), frozen for migration regression checks.
-- view: chunks_current
      WITH version_candidates AS (
        SELECT file_id, content_version, MAX(ingested_at) AS version_ingested_at
        FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks`
        WHERE content_version IS NOT NULL
        GROUP BY file_id, content_version
      ),
      latest_versions AS (
        SELECT file_id, content_version
        FROM version_candidates
        QUALIFY ROW_NUMBER() OVER (
          PARTITION BY file_id ORDER BY version_ingested_at DESC, content_version DESC
        ) = 1
      ),
      deduplicated_chunks AS (
        SELECT file_id, chunk_index, content, embedding_status, content_version, ingested_at, is_deleted,
          reflection_start_date, reflection_end_date, document_name, drive_url,
          reflection_month, reflection_sequence
        FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks`
        QUALIFY ROW_NUMBER() OVER (
          PARTITION BY file_id, content_version, chunk_index ORDER BY ingested_at DESC
        ) = 1
      )
      SELECT chunks.file_id, chunks.chunk_index, chunks.content, chunks.embedding_status,
        chunks.content_version, chunks.ingested_at,
        chunks.reflection_start_date, chunks.reflection_end_date, chunks.document_name, chunks.drive_url,
        chunks.reflection_month, chunks.reflection_sequence,
        chunks.reflection_start_date AS retro_start_date,
        chunks.reflection_end_date AS retro_end_date,
        chunks.reflection_month AS retro_month,
        chunks.reflection_sequence AS retro_sequence
      FROM deduplicated_chunks AS chunks
      INNER JOIN latest_versions USING (file_id, content_version)
      WHERE COALESCE(chunks.is_deleted, FALSE) = FALSE

-- view: silver_chunks
      SELECT
        file_id,
        chunk_index,
        content,
        content_version,
        ingested_at,
        reflection_start_date,
        reflection_end_date,
        document_name,
        drive_url,
        reflection_month,
        reflection_sequence,
        retro_start_date,
        retro_end_date,
        retro_month,
        retro_sequence,
        DATE(ingested_at, "Asia/Tokyo") AS ingested_date,
        embedding_status,
        embedding_status = "completed" AS is_embedded,
        LENGTH(content) AS content_length
      FROM `${var.project_id}.${google_bigquery_dataset.etl.dataset_id}.chunks_current`

-- view: retro_documents_legacy
      WITH documents AS (
        SELECT
          file_id,
          STRING_AGG(content, '\n\n' ORDER BY chunk_index) AS content,
          MAX(ingested_at) AS ingested_at,
          ARRAY_AGG(
            STRUCT(
              document_name,
              drive_url,
              content_version,
              reflection_month,
              reflection_sequence,
              reflection_start_date,
              reflection_end_date
            )
            ORDER BY ingested_at DESC, chunk_index ASC
            LIMIT 1
          )[OFFSET(0)] AS metadata
        FROM `${var.project_id}.${google_bigquery_dataset.silver.dataset_id}.silver_chunks`
        GROUP BY file_id
      )
      SELECT
        file_id,
        metadata.document_name,
        metadata.drive_url,
        content,
        metadata.content_version,
        ingested_at,
        metadata.reflection_month,
        metadata.reflection_sequence,
        metadata.reflection_start_date,
        metadata.reflection_end_date
      FROM documents
