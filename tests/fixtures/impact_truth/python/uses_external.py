import external as ext


def caller(items):
    return [ext.build(i) for i in items]
