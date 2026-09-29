# classes.ps1 - enum, class, inheritance, constructor and methods.

enum Color {
    Red
    Green = 2
    Blue
}

class Animal {
    [string]$Name
    hidden [int]$Age = 3
    static [int]$Count

    Animal([string]$name) {
        $this.Name = $name
        Register-Animal -Name $name
    }

    [string] Speak([int]$times) {
        return "..."
    }

    static [Animal] Create([string]$n) {
        return [Animal]::new($n)
    }
}

class Dog : Animal {
    [string] Speak(
        [int]$times
    ) {
        $this.Bark()
        return "Woof"
    }

    [void] Bark() { Write-Host 'woof' }
}
