load("//tools:defs.bzl", "pkg_tar", "py_library", "py_requirement", "py_test")

package(
    default_visibility = ["//benchmarks:__subpackages__"],
    licenses = ["notice"],
)

py_library(
    name = "redisbenchmark",
    srcs = ["__init__.py"],
)

py_test(
    name = "redisbenchmark_test",
    srcs = ["redisbenchmark_test.py"],
    python_version = "PY3",
    deps = [
        ":redisbenchmark",
        py_requirement("attrs", False),
        py_requirement("atomicwrites", False),
        py_requirement("more-itertools", False),
        py_requirement("pathlib2", False),
        py_requirement("pluggy", False),
        py_requirement("py", False),
        py_requirement("pytest", True),
        py_requirement("six", False),
    ],
)

pkg_tar(
    name = "tar",
    srcs = [
        "Dockerfile",
    ],
)
