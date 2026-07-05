variable "cluster_id" {
  type     = string
  nullable = false
}

# import CSV files from S3 with role arn
resource "tidbcloud_dedicated_import" "example_s3" {
  cluster_id = var.cluster_id
  import_options = {
    file_type = "CSV"
  }
  source = {
    type = "S3"
    s3 = {
      uri       = "s3://your-bucket/your-path/"
      auth_type = "ROLE_ARN"
      role_arn  = "arn:aws:iam::123456789012:role/your-role"
    }
  }
}

# import SQL files from GCS with service account key, replacing duplicate records
resource "tidbcloud_dedicated_import" "example_gcs" {
  cluster_id = var.cluster_id
  import_options = {
    file_type            = "SQL"
    duplication_handling = "REPLACE"
  }
  source = {
    type = "GCS"
    gcs = {
      uri                 = "gs://your-bucket/your-path/"
      auth_type           = "SERVICE_ACCOUNT_KEY"
      service_account_key = "your-service-account-key"
    }
  }
}

# import CSV files from Azure Blob into a specific table
resource "tidbcloud_dedicated_import" "example_azure_blob" {
  cluster_id = var.cluster_id
  import_options = {
    file_type = "CSV"
  }
  source = {
    type = "AZURE_BLOB"
    azure_blob = {
      uri       = "https://youraccount.blob.core.windows.net/your-container/your-path/"
      auth_type = "SAS_TOKEN"
      sas_token = "your-sas-token"
    }
  }
  target_table_infos = [
    {
      schema      = "your-database"
      table       = "your-table"
      custom_file = "your-file.csv"
    }
  ]
}
