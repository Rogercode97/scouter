class Database:
    def __init__(self, host: str):
        self.host = host

    def connect(self):
        print(f"Connecting to {self.host}")

def standalone_function():
    return 42
