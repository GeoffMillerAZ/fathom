from checkov.terraform.checks.resource.base_resource_check import BaseResourceCheck


class EarlyCheck(BaseResourceCheck):
    def __init__(self):
        name = "A check before the error"
        id = "CKV_EARLY_1"
        supported_resources = ["aws_early"]
        super().__init__(name=name, id=id, categories=[], supported_resources=supported_resources)


class LateCheck(BaseResourceCheck):
    def __init__(self:
        id = "CKV_LATE_1"
        supported_resources = ["aws_late"
