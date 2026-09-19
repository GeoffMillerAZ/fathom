import { add } from "./math";

export class Greeter {
    greet(name: string): number {
        console.log(name);
        return add(1, 2) + name.length;
    }
}
