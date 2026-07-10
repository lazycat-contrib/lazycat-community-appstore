# Software source trust boundary

A user-added software source is fetched with the client service's own network reach. A source URL can therefore contact loopback services and private-network services that are reachable by the client process.

Loopback and private-network sources are intentionally supported for the current single-user and LazyCat deployment model. The active policy is `allowSourceURLPolicy`; it validates HTTP/HTTPS URL shape but does not claim or provide administrator-only enforcement.

A future multi-user or OIDC deployment can provide an administrator-only `sourceURLPolicy` implementation once the identity model exposes trustworthy roles and authorization decisions. No configuration flag is exposed before that role model exists, because a flag without enforceable identity would imply a security boundary that the service cannot actually guarantee.
