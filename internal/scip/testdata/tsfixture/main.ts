import { Greeter } from "./greeter";

export function run(): number {
    const g = new Greeter();
    return g.greet("hi");
}
