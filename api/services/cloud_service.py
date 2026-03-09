# api/services/cloud_service.py
# Facade consolidating all cloud provider operations.
# Routers and workers import this instead of individual cloud integration modules.

from api.integrations.aws import AWSClient, assume_role, list_accounts
from api.integrations.gcp import GCPClient, list_projects
from api.integrations.azure import AzureClient, list_subscriptions
from api.models.cloud import CloudAccount, CloudCredential
from api.utils.credentials import decrypt_secret, validate_credentials


class CloudService:
    """Facade aggregating cloud provider operations.

    Consumers import only this class instead of individual integration modules,
    keeping router/worker fan-out within acceptable bounds.
    """

    def __init__(self) -> None:
        self._aws = AWSClient()
        self._gcp = GCPClient()
        self._azure = AzureClient()

    # --- AWS ---

    def assume_role(self, role_arn: str, session_name: str):
        return assume_role(self._aws, role_arn, session_name)

    def list_aws_accounts(self):
        return list_accounts(self._aws)

    # --- GCP ---

    def list_gcp_projects(self):
        return list_projects(self._gcp)

    # --- Azure ---

    def list_azure_subscriptions(self):
        return list_subscriptions(self._azure)

    # --- Credential utilities ---

    def validate(self, credential: CloudCredential) -> bool:
        decrypted = decrypt_secret(credential.secret)
        return validate_credentials(credential.provider, decrypted)
