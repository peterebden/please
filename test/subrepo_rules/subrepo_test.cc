// This really just tests that it's possible to compile gtest OK as a subrepo.

#include "gtest/gtest.h"

int multiply(x, y) {
  return x * y;
}

TEST(SubrepoTest, BasicTest) {
  EXPECT_TRUE(42, multiply(6, 7));
}
