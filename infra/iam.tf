data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "api" {
  name               = "${var.project_name}-${var.environment}-api"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role" "durable" {
  name               = "${var.project_name}-${var.environment}-durable"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy_attachment" "api_logs" {
  role       = aws_iam_role.api.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy_attachment" "durable_logs" {
  role       = aws_iam_role.durable.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy_attachment" "durable_execution" {
  role       = aws_iam_role.durable.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicDurableExecutionRolePolicy"
}

data "aws_iam_policy_document" "api" {
  statement {
    sid     = "Dynamo"
    actions = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:DeleteItem", "dynamodb:Query", "dynamodb:Scan"]
    resources = [
      aws_dynamodb_table.store.arn,
      "${aws_dynamodb_table.store.arn}/index/*",
    ]
  }

  statement {
    sid = "InvokeDurable"
    actions = [
      "lambda:InvokeFunction",
      "lambda:SendDurableExecutionCallbackSuccess",
      "lambda:SendDurableExecutionCallbackFailure",
    ]
    resources = [
      aws_lambda_function.durable.arn,
      "${aws_lambda_function.durable.arn}:*",
    ]
  }
}

data "aws_iam_policy_document" "durable_data" {
  statement {
    sid     = "Dynamo"
    actions = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:DeleteItem", "dynamodb:Query", "dynamodb:Scan"]
    resources = [
      aws_dynamodb_table.store.arn,
      "${aws_dynamodb_table.store.arn}/index/*",
    ]
  }
}

resource "aws_iam_role_policy" "api" {
  name   = "api-inline"
  role   = aws_iam_role.api.id
  policy = data.aws_iam_policy_document.api.json
}

resource "aws_iam_role_policy" "durable" {
  name   = "durable-inline"
  role   = aws_iam_role.durable.id
  policy = data.aws_iam_policy_document.durable_data.json
}
