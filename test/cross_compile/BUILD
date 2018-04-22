subinclude('//build_defs:plz_e2e_test')

genrule(
    name = 'record_arch',
    outs = ['arch.txt'],
    cmd = 'echo ${OS}_${ARCH} > $OUT',
)

sh_test(
    name = 'test_arch',
    src = 'test_arch.sh',
    data = [':record_arch'],
    labels = ['manual'],
)

sh_test(
    name = 'test_auto_arch',
    src = 'test_arch.sh',
    data = ['@test_x86:record_arch'],
)

plz_e2e_test(
    name = 'cross_compile_test',
    cmd = 'plz test -a test_x86 //test/cross_compile:test_arch',
)
