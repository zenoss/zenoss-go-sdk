ARG ZENKIT_BUILD_VERSION=1.17.0

FROM zenoss/zenkit-build:${ZENKIT_BUILD_VERSION} as builder

ENTRYPOINT ["/bin/bash"]