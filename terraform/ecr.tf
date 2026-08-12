resource "aws_ecr_repository" "app" {
  name = "chaos-gym-app"

  # Solo learning project — deleting the repo should not require manually
  # emptying it first.
  force_delete = true

  image_scanning_configuration {
    scan_on_push = true
  }
}

# Keep storage near zero: expire all but the most recent handful of images.
resource "aws_ecr_lifecycle_policy" "app" {
  repository = aws_ecr_repository.app.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep only the 5 most recent images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 5
      }
      action = { type = "expire" }
    }]
  })
}

# The chaos scheduler ships as its own image. A separate repository rather than
# another tag in the app's: the lifecycle rule above expires all but the five
# most recent images, so sharing one repository would eventually delete the
# scheduler because the app churned.
resource "aws_ecr_repository" "scheduler" {
  name         = "chaos-gym-scheduler"
  force_delete = true

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "scheduler" {
  repository = aws_ecr_repository.scheduler.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep only the 5 most recent images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 5
      }
      action = { type = "expire" }
    }]
  })
}

# The instance pulls images, so it needs read access — but only to these two
# repositories, not every repository in the account.
data "aws_iam_policy_document" "ecr_pull" {
  statement {
    actions = [
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage",
      "ecr:BatchCheckLayerAvailability",
    ]
    resources = [
      aws_ecr_repository.app.arn,
      aws_ecr_repository.scheduler.arn,
    ]
  }

  # GetAuthorizationToken has no resource to scope to — it is account-wide by
  # design, which is why the pull actions above are scoped instead.
  statement {
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ecr_pull" {
  name   = "ecr-pull-app"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.ecr_pull.json
}

output "ecr_repository_url" {
  description = "Push target for the app image"
  value       = aws_ecr_repository.app.repository_url
}

output "ecr_scheduler_repository_url" {
  description = "Push target for the chaos scheduler image"
  value       = aws_ecr_repository.scheduler.repository_url
}
