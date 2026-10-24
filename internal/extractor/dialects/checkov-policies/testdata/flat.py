from checkov.common.models.enums import CheckResult
from checkov.terraform.checks.resource.base_resource_check import BaseResourceCheck


class FlatCheck(BaseResourceCheck):
    def __init__(self):
        id = "CKV_FLAT_1"
        supported_resources = ["aws_flat"]
        super().__init__(name="A check at the repository root", id=id, categories=[], supported_resources=supported_resources)

    def scan_resource_conf(self, conf):
        return CheckResult.PASSED
