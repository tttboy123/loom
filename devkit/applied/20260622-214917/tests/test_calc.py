# tests/test_calc.py
import unittest
from src.calc import add


class TestAdd(unittest.TestCase):

    def test_add_two_positive(self):
        """旗舰用例：两个正数相加"""
        self.assertEqual(add(3, 5), 8)

    def test_add_positive_and_negative(self):
        """正数与负数相加"""
        self.assertEqual(add(10, -4), 6)

    def test_add_two_negatives(self):
        """两个负数相加"""
        self.assertEqual(add(-7, -2), -9)

    def test_add_zero(self):
        """与零相加"""
        self.assertEqual(add(0, 0), 0)
        self.assertEqual(add(5, 0), 5)


if __name__ == '__main__':
    unittest.main()
