import base64
hashes = {
    'balanced-match 1.0.2': '3oSeUBiayEn4gUC1L6bUB+pBVfp8A1c6R/VYz4SYVS6BGQB7W9DqHd1bRFTKFTnzsX3kpiT+bnniNH4PVQVQ==',
    'minimatch 10.0.1':     'zuXxBmHYFJRr3XVFpqB1FmCgqHjIBWAScDJg9bBuTe5eIhtLx+BTm7ZKiakVq7MPrJQCBijMUeaVOWFIHJAaA==',
    'brace-expansion 2.0.1':'XnAIvQ8ewe1ccfupLE2+szUkFL8zkFoFGOauhBLTg6Mzv5ghcdvEiDQMmZPC/iFpANJbHoEX8M3Ny+eNSWKdg==',
}
for name, b64 in hashes.items():
    total = len(b64)
    data = len(b64.rstrip('='))
    pad = b64.count('=')
    valid_len = (total % 4 == 0)
    try:
        decoded = base64.b64decode(b64 + '=' * (-total % 4))
        dec_bytes = len(decoded)
        ok = ''
    except Exception as e:
        dec_bytes = 0
        ok = f' ERROR: {e}'
    print(f'{name}: total={total} data={data} pad={pad} div4={valid_len} decoded={dec_bytes}B{ok}')
