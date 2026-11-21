output "cloudfront_url" {
  description = "URL pública da loja (S3 + API via CloudFront)."
  value       = "https://${aws_cloudfront_distribution.frontend.domain_name}"
}

output "api_endpoint" {
  description = "HTTP API direta (útil para debug). O site usa o caminho /api no CloudFront."
  value       = aws_apigatewayv2_api.http.api_endpoint
}

output "frontend_bucket" {
  description = "Bucket S3 do frontend para aws s3 sync."
  value       = aws_s3_bucket.frontend.id
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.frontend.id
}

output "durable_function_name" {
  value = aws_lambda_function.durable.function_name
}

output "dynamodb_table" {
  value = aws_dynamodb_table.store.name
}
