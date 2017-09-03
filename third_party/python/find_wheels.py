"""Script to find & download multiple versions of a package.

We attempt to simultaneously support Python 2.7, 3.4, 3.5 and 3.6 (or more
or less any version in future, but we draw the line at the older ones); for pure
Python code this is fine, but for C extensions that we package in please_pex
(notably coverage, but also cffi that we use for bootstrap) it's useful to be
able to download multiple versions of the extension. Python theoretically
supports this via suffixes, although in practice it's not that trivial.
"""

import os
import platform
import re
import sys

import requests


PIP_ARCHITECTURES = ['cp27-cp27m', 'cp34-cp34m', 'cp35-cp35m', 'cp36-cp36m']
PIP_PLATFORMS = {'darwin':  'macosx', 'linux': 'manylinux1'}
PIP_URL_BASE = 'https://pypi.python.org/simple/'


def find_wheels(package_spec):
    package_name, _, version = package_spec.partition('==')
    response = requests.get(PIP_URL_BASE + package_name)
    response.raise_for_status()
    # It's hard to know what the architecture should be; somewhat arbitrarily
    # the description seems to change per-platform. Right now we don't support
    # any non-amd64 architectures so we can just assume that, more or less.
    archs = '|'.join(PIP_ARCHITECTURES)
    plat = PIP_PLATFORMS.get(sys.platform, sys.platform)
    link_re = re.compile('<a href="([^"]+%s-%s-(?:%s)-%s[^"]+(?:x86_64|amd64|intel).whl)(?:#.*)"' % (package_name, version, archs, plat))
    for match in link_re.findall(response.text):
        yield os.path.join(PIP_URL_BASE, package_name, match)


if __name__ == '__main__':
    for url in find_wheels(sys.argv[1]):
        print(url)
