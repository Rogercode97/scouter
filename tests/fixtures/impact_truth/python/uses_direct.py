from core import scale, transform


def direct(items):
    return [scale(transform(i)) for i in items]
