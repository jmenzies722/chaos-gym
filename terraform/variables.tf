variable "aws_region" {
  description = "AWS region for all chaos-gym resources"
  type        = string
  default     = "us-east-1"
}

variable "alert_email" {
  description = "Email address that receives the monthly budget alert"
  type        = string
}

variable "monthly_budget_usd" {
  description = "Monthly spend threshold that triggers the alert"
  type        = number
  default     = 20
}

variable "vpc_cidr" {
  description = "IP range for the chaos-gym VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidr" {
  description = "IP range for the single public subnet the instance lives in"
  type        = string
  default     = "10.0.1.0/24"
}

variable "instance_type" {
  description = "EC2 instance type for the k3s node"
  type        = string
  # t3.micro was tried first and measurably failed: CPUCreditBalance sat at 0
  # through the whole k3s install, throttling the box so hard that SSM commands
  # queued instead of running. t3.small fixes the CPU but its 2 GiB is under
  # the ~2.5 GiB that k3s + kube-prometheus-stack + OTel Collector need, so
  # t3.medium (4 GiB) is the real floor.
  default = "t3.medium"
}

variable "root_volume_gb" {
  description = "Root EBS volume size for the k3s node"
  type        = number
  default     = 20
}
