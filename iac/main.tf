terraform {
  # The state path is supplied by scripts/terraform from a shared, ignored
  # directory outside each Git worktree. Do not run Terraform directly.
  backend "local" {}

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.location
}
