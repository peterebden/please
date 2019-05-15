#include "test/cc_rules/multisrc.h"

#include <gflags/gflags.h>

DEFINE_integer(test, 42, "Test flag");

TEST(GFlagsBasic) {
  auto info = gflags::GetCommandLineFlagInfoOrDie("test");
  CHECK_EQUAL("42", info.default_value);
}
