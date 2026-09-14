# OpenGL
[Fireball](https://github.com/fireball-lang/fireball) library and generator for OpenGL.

## Features
- Generates instance-based `Api` struct instead of global state.
- Parses enums into strongly typed, distinct enums instead of untyped integers (`GL_VERTEX_SHADER` -> `ShaderType::VertexShader`).
- Function names have the `gl` prefix stripped and are converted to snake case (`glCreateBuffers` -> `create_buffers`). 
- Dynamically queries the version + extensions (`KHR`, `ARB` and `EXT`) and fallbacks to a panic stub.

## Usage
```fireball
func get_gl_address(name: StringView) *void {
    // `glfw_get_proc_address()` or `SDL_GL_GetProcAddress()` for example
}

func main() {
    // window / context init

    var gl = opengl::Api::load(get_gl_address).unwrap();
    
    var buffer: u32;
    gl.create_buffers(1, &buffer);
}
```
