load("//tools:defs.bzl", "go_library")

package(licenses = ["notice"])

go_library(
    name = "netfilter",
    srcs = [
        "netfilter.go",
    ],
    # This target depends on netstack and should only be used by epsocket,
    # which is allowed to depend on netstack.
    visibility = ["//pkg/sentry:internal"],
    deps = [
        "//pkg/abi/linux",
        "//pkg/binary",
        "//pkg/log",
        "//pkg/sentry/kernel",
        "//pkg/syserr",
        "//pkg/tcpip",
        "//pkg/tcpip/iptables",
        "//pkg/tcpip/stack",
        "//pkg/usermem",
    ],
)
