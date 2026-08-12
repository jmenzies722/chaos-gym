data "aws_iam_policy_document" "budgets_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["budgets.amazonaws.com"]
    }
  }
}

# Scoped to this one instance by ARN. A budget action that could stop any
# instance in the account would be a worse guardrail than none — it would put
# unrelated projects sharing this account in the blast radius.
data "aws_iam_policy_document" "stop_k3s_instance" {
  statement {
    actions   = ["ec2:StopInstances"]
    resources = ["arn:aws:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:instance/${aws_instance.k3s.id}"]
  }

  statement {
    actions   = ["ec2:DescribeInstanceStatus"]
    resources = ["*"]
  }

  statement {
    actions   = ["ssm:StartAutomationExecution", "ssm:GetAutomationExecution"]
    resources = ["*"]
  }
}

data "aws_caller_identity" "current" {}

resource "aws_iam_role" "budget_action" {
  name               = "chaos-gym-budget-action"
  assume_role_policy = data.aws_iam_policy_document.budgets_assume_role.json
}

resource "aws_iam_role_policy" "budget_action" {
  name   = "stop-k3s-instance"
  role   = aws_iam_role.budget_action.id
  policy = data.aws_iam_policy_document.stop_k3s_instance.json
}

resource "aws_budgets_budget_action" "stop_instance" {
  budget_name        = aws_budgets_budget.monthly_cost_alert.name
  action_type        = "RUN_SSM_DOCUMENTS"
  approval_model     = "AUTOMATIC"
  notification_type  = "ACTUAL"
  execution_role_arn = aws_iam_role.budget_action.arn

  action_threshold {
    action_threshold_type  = "PERCENTAGE"
    action_threshold_value = 100
  }

  definition {
    ssm_action_definition {
      action_sub_type = "STOP_EC2_INSTANCES"
      instance_ids    = [aws_instance.k3s.id]
      region          = var.aws_region
    }
  }

  subscriber {
    address           = var.alert_email
    subscription_type = "EMAIL"
  }
}
