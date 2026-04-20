interface User {
  id: number;
  name: string;
}

/**
 * AuthService handles user authentication and session management.
 * It manages a list of users and their login states.
 */
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
