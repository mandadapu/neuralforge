# api/services/cloud_service.py
# Facade consolidating all cloud provider operations.
# Routers and workers import this instead of individual cloud integration modules.

import logging

from api.integrations.aws import AWSClient, assume_role, list_accounts
from api.integrations.gcp import GCPClient, list_projects
from api.integrations.azure import AzureClient, list_subscriptions
from api.models.cloud import CloudCredential
from api.utils.credentials import decrypt_secret, validate_credentials

_audit_log = logging.getLogger("audit")


class CloudService:
    """Facade aggregating cloud provider operations.

    Consumers import only this class instead of individual integration modules,
    keeping router/worker fan-out within acceptable bounds.

    All mutating and credential-touching operations are recorded in the audit
    log (HIPAA §164.312(b), SOX §404, GDPR Art. 30).
    """

    def __init__(self) -> None:
        self._aws = AWSClient()
        self._gcp = GCPClient()
        self._azure = AzureClient()

    # --- AWS ---

    def assume_role(self, role_arn: str, session_name: str):
        _audit_log.info("cloud.assume_role provider=aws role_arn=%s session=%s", role_arn, session_name)
        return assume_role(self._aws, role_arn, session_name)

    def list_aws_accounts(self):
        _audit_log.info("cloud.list_accounts provider=aws")
        return list_accounts(self._aws)

    # --- GCP ---

    def list_gcp_projects(self):
        _audit_log.info("cloud.list_accounts provider=gcp")
        return list_projects(self._gcp)

    # --- Azure ---

    def list_azure_subscriptions(self):
        _audit_log.info("cloud.list_accounts provider=azure")
        return list_subscriptions(self._azure)

    # --- Credential utilities ---

    def validate(self, credential: CloudCredential) -> bool:
        _audit_log.info("cloud.validate_credential provider=%s", credential.provider)
        decrypted = decrypt_secret(credential.secret)
        return validate_credentials(credential.provider, decrypted)
