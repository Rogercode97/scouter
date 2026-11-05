from functools import partial

import helpers

from core import boost


def via_map(items):
    return list(map(boost, items))


def via_partial(value):
    bumped = partial(boost)
    return bumped(value)


def via_helpers_map(items):
    return list(helpers.map(boost, items))


def via_lambda_partial(value):
    bumped = partial(lambda x: x, boost)
    return bumped(value)
