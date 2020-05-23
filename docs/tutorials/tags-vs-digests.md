# Tags Vs. Digests
Images can be referenced by tag or digest. For instance, at the time of
writing this README, the official most recent version of the python 3.6
image on Dockerhub could be specified by tag, as in `python:3.6`, or by
digest of the image's contents, as in
`python@sha256:25a189a536ae4d7c77dd5d0929da73057b85555d6b6f8a66bfbcc1a7a7de094b`.

Images referenced by tag are mutable. The python maintainers could push a new
image to Dockerhub with the same 3.6 tag. Downstream applications that
required the previous `python:3.6` image could break.

Images referenced by digest are immutable. Despite having the same tag,
a newly pushed image will have a new digest. The previous image can still
be referenced by the previous digest.

When deploying to Kubernetes, digests make it easy to rollback broken
deployments. If your previous, working deployment relied on `myimage@sha256:2273f9a536ae4d7c77d6h49k29da73057b85555d6b6f8a66bfbcc1a7a7de094b`
and the broken, updated deployment relies on
`myimage@sha256:92038492583f9a3a4d7c77d6h49k29057b85555d6b6f8a66bfbcc1a7a7d1947f`,
rolling back to the working deployment would be as easy as changing the digest
back to the previous digest. Alternatively, if the previous, working deployment
relied on `myimage:v1` and the broken, updated image relies on  `myimage:v1`,
it would be more challenging to rollback by distinguishing between the images.

Although digests solve mutability problems, manually specifying them comes
with a host of problems. Namely:
* Applications will no longer benefit from updates (security updates,
performance updates, etc.).
* Dockerfiles and docker-compose files will become stale.
* Digests are considerably less readable than tags.
* Keeping digests up-to-date can become unwieldly in projects with many
services.
* Specifying the correct digest is complicated. Local digests may differ
from remote digests, and there are many different types of digests
(manifest digests, layer digests, etc.)

`docker-lock` solves all of these problems by storing digests in a Lockfile,
allowing developers to simply use tags since digests are recorded
in the background.