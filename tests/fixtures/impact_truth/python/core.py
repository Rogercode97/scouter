def transform(value):
    return value * 2


def scale(value):
    return value * 10


def boost(value):
    return value + 1


def pipeline(items):
    return [scale(transform(i)) for i in items]
