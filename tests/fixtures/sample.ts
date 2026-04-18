interface User {
  id: number;
  name: string;
}

class AuthService {
  private users: User[] = [];

  login(username: string): boolean {
    return true;
  }
}

function globalHelper() {
  console.log("Helper");
}

const arrowFunc = (x: number) => x * 2;
