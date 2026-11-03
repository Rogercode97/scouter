import core as c


def aliased(items):
    return [c.boost(i) for i in items]
