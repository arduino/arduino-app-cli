import sys
import textwrap


def to_jomla_matrix(a, b, c, d):
    s = f"{a:032b}{b:032b}{c:032b}{d:032b}"[: 12 * 8]
    a = textwrap.wrap(s, 12)
    b = [x + "0" for x in a]
    for x in textwrap.wrap("".join(b), 32):
        rev = x[::-1]
        print(hex(int(rev, 2)))


def main():
    if len(sys.argv) != 5:
        print("Usage: convert_matrix.py <a> <b> <c> <d>")
        return

    to_jomla_matrix(
        int(sys.argv[1], 16),
        int(sys.argv[2], 16),
        int(sys.argv[3], 16),
        int(sys.argv[4], 16),
    )


if __name__ == "__main__":
    main()
