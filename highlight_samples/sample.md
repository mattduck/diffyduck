# Sample Markdown for Syntax Highlighting

This is a sample **Markdown** file demonstrating _various_ formatting options.

## Headers

### Third Level Header
#### Fourth Level Header
##### Fifth Level Header

## Text Formatting

This is **bold text** and this is *italic text*.
You can also use __bold__ and _italic_ with underscores.
Here's some ~~strikethrough~~ text.

## Lists

### Unordered List
- Item one
- Item two
  - Nested item
  - Another nested item
- Item three

### Ordered List
1. First item
2. Second item
   1. Nested numbered
   2. Another nested
3. Third item

### Task List
- [x] Completed task
- [ ] Incomplete task
- [ ] Another task

## Links and Images

[Link to GitHub](https://github.com)
[Reference link][ref]

![Alt text for image](image.png)

[ref]: https://example.com "Example Site"

## Code

Inline `code` looks like this.

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

```python
def greet(name):
    return f"Hello, {name}!"

print(greet("World"))
```

## Blockquotes

> This is a blockquote.
> It can span multiple lines.
>
> > Nested blockquotes are also possible.

## Tables

| Header 1 | Header 2 | Header 3 |
|----------|:--------:|---------:|
| Left     | Center   | Right    |
| Cell     | Cell     | Cell     |

## Horizontal Rules

---

***

## Footnotes

Here's a sentence with a footnote[^1].

[^1]: This is the footnote content.

## HTML in Markdown

<details>
<summary>Click to expand</summary>

This is hidden content that can be revealed.

</details>

<div align="center">
  Centered content using HTML
</div>

## Math (if supported)

Inline math: $E = mc^2$

Block math:
$$
\sum_{i=1}^{n} x_i = x_1 + x_2 + \cdots + x_n
$$
