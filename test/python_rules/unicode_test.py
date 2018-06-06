# -*- coding: utf-8 -*-
"""A test that please can run python with unicode in the source."""

import unittest


class UnicodeTest(unittest.TestCase):
    def test_unicode(self):
        self.assertEqual(u'kérem', u'kérem')
