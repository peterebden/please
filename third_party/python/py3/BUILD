# We go through some gymnastics here to try to install multiple versions of binary
# artifacts so we can support python3.4, 3.5 and 3.6 simultaneously.
package(default_python_interpreter = 'python3')

# We should ideally not use stuff like this at parse time but it's hard to get away without
# since we require the filenames to be different.
os = ('darwin' if (CONFIG.OS == 'darwin') else 'x86_64-linux-gnu')

python_wheel(
    name = 'coverage',
    version = '4.3.4',
    repo = 'https://get.please.build/third_party/python/py3',
    visibility = ['PUBLIC'],
)

python_wheel(
    name = 'cffi34',
    package_name = 'cffi',
    version = '1.10.0',
    outs = ['_cffi_backend.cpython-34m.so'],
    repo = 'https://get.please.build/third_party/python/py34',
)

python_wheel(
    name = 'cffi35',
    package_name = 'cffi',
    version = '1.10.0',
    outs = [
        '.libs_cffi_backend/libffi-72499c49.so.6.0.4' if (CONFIG.OS == 'linux') else None,
        '_cffi_backend.cpython-35m-%s.so' % os,
        'cffi',
    ],
    repo = 'https://get.please.build/third_party/python/py3',
)

python_wheel(
    name = 'cffi36',
    package_name = 'cffi',
    version = '1.10.0',
    outs = ['_cffi_backend.cpython-36m-%s.so' % os],
    repo = 'https://get.please.build/third_party/python/py36',
)

filegroup(
    name = 'cffi',
    srcs = [
        ':cffi34',
        ':cffi35',
        ':cffi36',
    ],
    visibility = ['PUBLIC'],
)
