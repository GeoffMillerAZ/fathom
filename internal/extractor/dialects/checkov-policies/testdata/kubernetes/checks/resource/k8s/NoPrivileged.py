from typing import Any, Dict

from checkov.common.models.enums import CheckResult
from checkov.kubernetes.checks.resource.base_container_check import BaseK8sContainerCheck


class _AbstractContainerCheck(BaseK8sContainerCheck):
    """A helper with no id: not a check of its own."""

    def helper(self) -> bool:
        return True


class NoPrivileged(BaseK8sContainerCheck):
    def __init__(self) -> None:
        name = "Containers should not run privileged"
        id = "CKV_K8S_16"
        # the framework's base class supplies the entity kinds
        super().__init__(name=name, id=id)

    def scan_container_conf(self, metadata: Dict[str, Any], conf: Dict[str, Any]) -> CheckResult:
        if conf.get("securityContext", {}).get("privileged"):
            return CheckResult.FAILED
        return CheckResult.PASSED


class NoPrivilegedKinds(BaseK8sContainerCheck):
    def __init__(self) -> None:
        name = "Pods should not run privileged"
        id = "CKV_K8S_17"
        supported_kind = ("Pod", "Deployment")
        super().__init__(name=name, id=id, supported_entities=supported_kind)


check = NoPrivileged()
