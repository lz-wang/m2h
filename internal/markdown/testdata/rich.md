# Rich Markdown

## Lists

- A
- B
  - B.1

1. One
2. Two
   1. Two.One

## Math

Inline: $E = mc^2$

Block:

$$
\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}
$$

## Mermaid

```mermaid
flowchart LR
    Markdown --> Goldmark
    Goldmark --> HTML
    HTML --> Browser
```

## Code

```go
func main() { println("hi") }
```
