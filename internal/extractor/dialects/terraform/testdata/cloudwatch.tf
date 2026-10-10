# CloudWatch as IaC (LB-F371): a metric filter's literal metric is an identity,
# a function's literal name TL-5's; an alarm's metric and a /aws/lambda/<name>
# log group are identity references; a variable-driven one names nothing.
resource "aws_lambda_function" "api" {
  function_name = "tf-api"
  filename      = data.archive_file.api.output_path
  handler       = "app.handler"
  runtime       = "python3.12"
}

data "archive_file" "api" {
  type        = "zip"
  source_dir  = "${path.module}/src"
  output_path = "${path.module}/api.zip"
}

resource "aws_lambda_function" "dynamic" {
  function_name = "${var.prefix}-fn"
  filename      = "${path.module}/single.zip"
  handler       = "index.handler"
  runtime       = "nodejs20.x"
}

resource "aws_lambda_function" "worker" {
  filename = data.archive_file.worker.output_path
  handler  = "worker.run"
  runtime  = var.runtime
}

data "archive_file" "worker" {
  type        = "zip"
  output_path = "${path.module}/worker.zip"

  source {
    filename = "worker.py"
    content  = file("${path.module}/fn/worker.py")
  }
}

resource "aws_cloudwatch_log_metric_filter" "errors" {
  name           = "errors"
  pattern        = "ERROR"
  log_group_name = "/aws/lambda/tf-api"

  metric_transformation {
    name      = "Errors"
    namespace = "TF/App"
    value     = "1"
  }
}

resource "aws_cloudwatch_log_metric_filter" "by_var" {
  name           = "by-var"
  pattern        = "WARN"
  log_group_name = aws_cloudwatch_log_group.app.name

  metric_transformation {
    name      = var.metric_name
    namespace = "TF/App"
    value     = "1"
  }
}

resource "aws_cloudwatch_log_group" "app" {
  name = "/aws/lambda/${aws_lambda_function.api.function_name}"
}

resource "aws_cloudwatch_metric_alarm" "errors" {
  alarm_name          = "errors"
  namespace           = "TF/App"
  metric_name         = "Errors"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  dimensions = {
    FunctionName = aws_lambda_function.api.function_name
  }
  alarm_actions = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "math" {
  alarm_name          = "math"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1

  metric_query {
    id          = "e1"
    expression  = "m1 * 2"
    return_data = true
  }

  metric_query {
    id = "m1"
    metric {
      namespace   = "TF/App"
      metric_name = "Errors"
      period      = 60
      stat        = "Sum"
    }
  }
}

resource "aws_sns_topic" "alerts" {
  name = "alerts"
}
