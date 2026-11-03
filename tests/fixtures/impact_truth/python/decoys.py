def transform(value):
    return value


def mention():
    return "transform is a function name"


# transform is called nowhere in this module except below
def caller_of_local(items):
    return [transform(i) for i in items]
