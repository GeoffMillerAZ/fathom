from checkov.common.models.enums import CheckCategories, CheckResult
from checkov.terraform.checks.resource.base_resource_check import BaseResourceCheck


class BucketVersioning(BaseResourceCheck):
    def __init__(self):
        super().__init__(
            name="Ensure buckets keep versions",
            id="CKV_CUSTOM_1",
            categories=(CheckCategories.BACKUP_AND_RECOVERY,),
            supported_resources=("aws_s3_bucket", "google_storage_bucket"),
        )

    def scan_resource_conf(self, conf):
        return CheckResult.PASSED


class WildcardTags(BaseResourceCheck):
    def __init__(self):
        name = "Ensure every resource is tagged"
        id = "CKV_CUSTOM_2"
        super().__init__(name=name, id=id, categories=[], supported_resources=["*"])

    def scan_resource_conf(self, conf):
        return CheckResult.PASSED if conf.get("tags") else CheckResult.FAILED


class ComputedId(BaseResourceCheck):
    def __init__(self, check_id):
        super().__init__(name="n", id=check_id, categories=[], supported_resources=["aws_x"])


class NotACheck:
    def __init__(self):
        id = "CKV_NOT_A_CHECK"
        supported_resources = ["aws_y"]
