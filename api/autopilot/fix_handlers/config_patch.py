"""
Config patch handler — applies configuration patches at runtime.
"""
from api.cors_utils import get_allowed_origins

cors_config = {"origins": get_allowed_origins()}
