terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {}

resource "aws_kms_key" "snapshots" {
  count                   = var.kms_key_arn == "" ? 1 : 0
  description             = "DRE snapshot encryption key"
  deletion_window_in_days = 7
  tags                    = var.tags
}

locals {
  kms_arn = var.kms_key_arn != "" ? var.kms_key_arn : aws_kms_key.snapshots[0].arn
}

resource "aws_s3_bucket" "snapshots" {
  bucket = var.bucket_name
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "snapshots" {
  bucket                  = aws_s3_bucket.snapshots.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "snapshots" {
  bucket = aws_s3_bucket.snapshots.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = local.kms_arn
    }
  }
}

data "aws_iam_policy_document" "collector_s3" {
  statement {
    actions = [
      "s3:PutObject",
      "s3:GetObject",
      "s3:DeleteObject",
      "s3:ListBucket",
    ]
    resources = [
      aws_s3_bucket.snapshots.arn,
      "${aws_s3_bucket.snapshots.arn}/*",
    ]
  }
  statement {
    actions   = ["kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey"]
    resources = [local.kms_arn]
  }
}

resource "aws_iam_policy" "collector_s3" {
  name   = "${var.cluster_name}-dre-collector-s3"
  policy = data.aws_iam_policy_document.collector_s3.json
}

data "aws_iam_policy_document" "collector_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [var.oidc_provider_arn]
    }
    condition {
      test     = "StringEquals"
      variable = "${var.oidc_provider_url}:sub"
      values   = ["system:serviceaccount:${var.namespace}:dre-collector"]
    }
    condition {
      test     = "StringEquals"
      variable = "${var.oidc_provider_url}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "collector" {
  name               = "${var.cluster_name}-dre-collector"
  assume_role_policy = data.aws_iam_policy_document.collector_assume.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "collector_s3" {
  role       = aws_iam_role.collector.name
  policy_arn = aws_iam_policy.collector_s3.arn
}
