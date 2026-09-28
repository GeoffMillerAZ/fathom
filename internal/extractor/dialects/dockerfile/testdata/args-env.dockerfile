FROM --platform=linux/amd64 debian:bookworm-slim AS base
ARG DEBUG
ENV LEGACY_STYLE production
ENV A=1 B=2
