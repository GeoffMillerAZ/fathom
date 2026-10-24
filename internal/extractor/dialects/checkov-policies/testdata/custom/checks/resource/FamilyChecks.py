from checkov.terraform.checks.resource.base_resource_check import BaseResourceCheck


class AbsPortCheck(BaseResourceCheck):
    def __init__(self, check_id, port):
        super().__init__(name="Port restricted", id=check_id, categories=[], supported_resources=["aws_security_group"])
        self.port = port


class Port22(AbsPortCheck):
    def __init__(self):
        super().__init__(check_id="CKV_FAM_22", port=22)


class Port80(AbsPortCheck):
    def __init__(self):
        check_id = "CKV_FAM_80"
        super().__init__(check_id=check_id, port=80)


class Helper(Port22):
    def __init__(self):
        self.id = "CKV_NOT_A_CHECK_EITHER"


class SingularNames(AbsPortCheck):
    def __init__(self):
        check_id = "CKV_FAM_1"
        supported_resource = ("aws_one",)
        supported_data = ["aws_two"]
        super().__init__(check_id=check_id)
