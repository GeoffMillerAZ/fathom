FROM ubuntu:24.04@sha256:68ee22004a9109b44973313b02ff7b1de250f1de0f5ecb628d73f579aaff33ac
RUN nice -n 10 sh -c 'apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends python3 libgnutls30t64 libssl3t64 libcurl4t64 libatomic1 libnuma1 ca-certificates'
