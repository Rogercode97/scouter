from functools import partial

from core import boost


def via_map(items):
    return list(map(boost, items))


def via_partial(value):
    bumped = partial(boost)
    return bumped(value)
