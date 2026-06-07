// Polyglot fixture: JavaScript

export function greet(name) {
  return "Hello " + name;
}

export const farewell = (name) => "Goodbye " + name;

class Greeter {
  greet(name) {
    return greet(name);
  }
}
