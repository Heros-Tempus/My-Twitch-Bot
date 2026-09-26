import os
import tkinter as tk
from tkinter import ttk
from tkinter import colorchooser
import urllib.parse
from pathlib import Path


def load_dotenv(path=None):
    """Load simple KEY=value pairs from a .env file without external dependencies."""
    env_path = Path(path) if path else Path(__file__).resolve().parent / ".env"
    values = {}

    if not env_path.exists():
        return values

    for raw_line in env_path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export "):].strip()
        if "=" not in line:
            continue

        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()

        if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
            value = value[1:-1]

        values[key] = value

    return values


class URLGeneratorApp:
    """A URL generator to encode query parameters for the chat overlay."""
    def __init__(self, root):
        self.root = root
        self.root.title("URL Generator")
        self.root.geometry("700x550")
        self.env = load_dotenv()
        self.env.update({k: v for k, v in os.environ.items() if k in {"DESKTOP_IP"}})

        vcmd_int = (self.root.register(self.validate_int), '%P')
        vcmd_alpha = (self.root.register(self.validate_alpha), '%P')

        base_frame = ttk.Frame(self.root, padding="10")
        base_frame.pack(fill=tk.X)
        ttk.Label(base_frame, text="Base URL:").pack(side=tk.LEFT)
        desktop_ip = self.env.get("DESKTOP_IP") or "localhost"
        self.base_url_var = tk.StringVar(value=f"http://{desktop_ip}:8080/chat.html")
        ttk.Entry(base_frame, textvariable=self.base_url_var).pack(side=tk.LEFT, fill=tk.X, expand=True, padx=5)

        self.main_frame = ttk.LabelFrame(self.root, text="Query Parameters (Optional)", padding="10")
        self.main_frame.pack(fill=tk.BOTH, expand=True, padx=10, pady=5)

        self.params = {
            "flow_bottom_up": {"type": "bool", "var": tk.StringVar(value="false")},
            "message_timeout_sec": {"type": "int", "var": tk.StringVar()},
            "max_messages": {"type": "int", "var": tk.StringVar()},
            "text_color": {"type": "hex", "var": tk.StringVar()},
            "text_size": {"type": "str", "var": tk.StringVar()},
            "text_font": {"type": "str", "var": tk.StringVar()},
            "text_weight": {"type": "int", "var": tk.StringVar()},
            "chat_bg": {"type": "rgba", "hex_var": tk.StringVar(), "alpha_var": tk.StringVar()},
            "chat_fade": {"type": "rgba", "hex_var": tk.StringVar(), "alpha_var": tk.StringVar()}
        }

        row = 0
        for param, config in self.params.items():
            ttk.Label(self.main_frame, text=f"{param}:").grid(row=row, column=0, sticky=tk.W, pady=5)
            
            p_type = config["type"]

            if p_type == "bool":
                widget = ttk.Checkbutton(
                    self.main_frame, text="Enable", 
                    variable=config["var"], onvalue="true", offvalue="false"
                )
                widget.grid(row=row, column=1, sticky=tk.W, padx=5, pady=5)

            elif p_type == "int":
                widget = ttk.Entry(self.main_frame, textvariable=config["var"], validate='key', validatecommand=vcmd_int)
                widget.grid(row=row, column=1, sticky=tk.EW, padx=5, pady=5)

            elif p_type == "hex":
                frame = ttk.Frame(self.main_frame)
                frame.grid(row=row, column=1, sticky=tk.EW, padx=5, pady=5)
                
                ttk.Entry(frame, textvariable=config["var"]).pack(side=tk.LEFT, fill=tk.X, expand=True)
                ttk.Button(frame, text="🎨 Pick", width=6, 
                          command=lambda v=config["var"]: self.pick_color(v)).pack(side=tk.RIGHT, padx=(5, 0))

            elif p_type == "rgba":
                frame = ttk.Frame(self.main_frame)
                frame.grid(row=row, column=1, sticky=tk.EW, padx=5, pady=5)
                
                ttk.Entry(frame, textvariable=config["hex_var"], width=12).pack(side=tk.LEFT)
                ttk.Button(frame, text="🎨 Pick", width=6, 
                          command=lambda v=config["hex_var"]: self.pick_color(v)).pack(side=tk.LEFT, padx=(5, 15))
                
                ttk.Label(frame, text="Alpha (0-1):").pack(side=tk.LEFT)
                ttk.Entry(frame, textvariable=config["alpha_var"], width=6, 
                         validate='key', validatecommand=vcmd_alpha).pack(side=tk.LEFT, padx=5)

            else:
                widget = ttk.Entry(self.main_frame, textvariable=config["var"])
                widget.grid(row=row, column=1, sticky=tk.EW, padx=5, pady=5)
                
            row += 1

        self.main_frame.columnconfigure(1, weight=1)

        btn_frame = ttk.Frame(self.root, padding="10")
        btn_frame.pack(fill=tk.X)
        
        ttk.Button(btn_frame, text="Generate URL", command=self.generate_url).pack(side=tk.LEFT, padx=5)
        ttk.Button(btn_frame, text="Copy to Clipboard", command=self.copy_to_clipboard).pack(side=tk.LEFT, padx=5)
        ttk.Button(btn_frame, text="Clear All", command=self.clear_fields).pack(side=tk.RIGHT, padx=5)

        out_frame = ttk.Frame(self.root, padding="10")
        out_frame.pack(fill=tk.BOTH, expand=True)
        self.output_text = tk.Text(out_frame, height=4, wrap=tk.CHAR)
        self.output_text.pack(fill=tk.BOTH, expand=True, pady=5)

    def validate_int(self, P):
        if P == "" or P.isdigit():
            return True
        return False

    def validate_alpha(self, P):
        if P in ("", "."): 
            return True
        try:
            val = float(P)
            return 0.0 <= val <= 1.0
        except ValueError:
            return False

    def pick_color(self, target_var):
        color = colorchooser.askcolor(title="Choose a Color")
        if color[1]: 
            target_var.set(color[1])

    def hex_to_rgb(self, hex_color):
        hex_color = hex_color.lstrip('#')
        if len(hex_color) == 3:
            hex_color = ''.join(c + c for c in hex_color)
        if len(hex_color) != 6:
            return (0, 0, 0)
        try:
            return tuple(int(hex_color[i:i+2], 16) for i in (0, 2, 4))
        except ValueError:
            return (0, 0, 0)

    def generate_url(self):
        query_dict = {}
        
        for param, config in self.params.items():
            if config["type"] == "rgba":
                hex_val = config["hex_var"].get().strip()
                alpha_val = config["alpha_var"].get().strip()
                
                if hex_val: 
                    r, g, b = self.hex_to_rgb(hex_val)
                    a = alpha_val if alpha_val else "1.0"
                    query_dict[param] = f"rgba({r},{g},{b},{a})"
            else:
                val = config["var"].get().strip()
                if val:
                    query_dict[param] = val

        encoded_query = urllib.parse.urlencode(query_dict, safe="(),")
        base_url = self.base_url_var.get().strip()
        final_url = f"{base_url}?{encoded_query}" if encoded_query else base_url

        self.output_text.delete(1.0, tk.END)
        self.output_text.insert(tk.END, final_url)

    def copy_to_clipboard(self):
        self.root.clipboard_clear()
        self.root.clipboard_append(self.output_text.get(1.0, tk.END).strip())
        self.root.update()

    def clear_fields(self):
        for config in self.params.values():
            if config["type"] == "rgba":
                config["hex_var"].set("")
                config["alpha_var"].set("")
            else:
                config["var"].set("")
        self.output_text.delete(1.0, tk.END)

if __name__ == "__main__":
    root = tk.Tk()
    app = URLGeneratorApp(root)
    root.mainloop()