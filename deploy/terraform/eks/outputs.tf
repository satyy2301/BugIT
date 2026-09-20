output "s3_bucket" {
  description = "Snapshot bucket name"
  value       = aws_s3_bucket.snapshots.id
}

output "s3_bucket_arn" {
  description = "Snapshot bucket ARN"
  value       = aws_s3_bucket.snapshots.arn
}

output "kms_key_arn" {
  description = "KMS key used for snapshot encryption"
  value       = local.kms_arn
}

output "collector_iam_role_arn" {
  description = "IRSA role ARN for dre-collector ServiceAccount"
  value       = aws_iam_role.collector.arn
}

output "helm_values_snippet" {
  description = "Paste into values-eks.yaml"
  value = <<-EOT
s3:
  bucket: ${aws_s3_bucket.snapshots.id}
  prefix: dre/snapshots
  region: ${data.aws_region.current.name}
collector:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: ${aws_iam_role.collector.arn}
EOT
}

data "aws_region" "current" {}
