"""Generate an Ed25519 keypair for actor-JWT signing.

    python -m nlq.keygen

Prints the base64 of the 32-byte seed (private) and the 32-byte public key, in
.env form. Go loads the seed via ed25519.NewKeyFromSeed; Python verifies with
the public key.
"""

from __future__ import annotations

import base64

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey


def main() -> None:
    priv = Ed25519PrivateKey.generate()
    seed = priv.private_bytes(
        serialization.Encoding.Raw,
        serialization.PrivateFormat.Raw,
        serialization.NoEncryption(),
    )
    pub = priv.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    print(f"NLQ_ACTOR_PRIVATE_KEY={base64.b64encode(seed).decode('ascii')}")
    print(f"NLQ_ACTOR_PUBLIC_KEY={base64.b64encode(pub).decode('ascii')}")


if __name__ == "__main__":
    main()
