fn hello_world() {
    println!("Hello, Scouter!");
}

struct Developer {
    name: String,
}

trait CodebaseIntelligence {
    fn analyze(&self);
}

impl Developer {
    fn code(&self) {
        hello_world();
    }
}
