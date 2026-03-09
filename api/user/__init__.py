"""
api/user — backward-compat re-export shim.

Formerly api/user_database.py (621 lines, 35 exports).
Split by responsibility: CRUD, auth, preferences.
"""
from api.user.crud import *
from api.user.auth import *
from api.user.preferences import *
