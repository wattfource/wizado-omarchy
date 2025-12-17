import json, re, pathlib, sys, shutil

path = pathlib.Path.home() / ".config/waybar/config.jsonc"

def get_terminal_cmd():
    # Helper to find a terminal command (basic version of bash logic)
    terms = ["ghostty", "alacritty", "kitty", "foot", "gnome-terminal", "konsole"]
    for t in terms:
        if shutil.which(t):
            if t == "gnome-terminal": return t + " --"
            if t == "konsole": return t + " -e"
            return t + " -e"
    return "alacritty -e"

try:
    print(f"Reading {path}")
    raw = path.read_text(encoding="utf-8")
    # Very small jsonc stripper (removes // comments). Omarchy config.jsonc is plain JSON today.
    raw2 = re.sub(r"//.*?$", "", raw, flags=re.M)
    cfg = json.loads(raw2)

    def inject_module(arr):
        if "custom/wizado" in arr:
            return arr
        if "custom/omarchy" in arr:
            i = arr.index("custom/omarchy") + 1
            return arr[:i] + ["custom/wizado"] + arr[i:]
        return arr + ["custom/wizado"]

    print("Injecting module...")
    cfg["modules-left"] = inject_module(cfg.get("modules-left", []))

    term_cmd = get_terminal_cmd()
    print(f"Detected terminal: {term_cmd}")

    cfg["custom/wizado"] = {
        "format": "{icon}",
        "return-type": "json",
        "exec": str(pathlib.Path.home() / ".config/waybar/scripts/wizado-status.sh"),
        "interval": 2,
        "on-click": term_cmd + " " + str(pathlib.Path.home() / ".local/share/steam-launcher/wizado-menu"),
        "on-click-right": str(pathlib.Path.home() / ".local/share/steam-launcher/enter-gamesmode") + " --mode tty",
        "on-click-middle": str(pathlib.Path.home() / ".local/share/steam-launcher/leave-gamesmode"),
        "tooltip": True
    }

    out = json.dumps(cfg, indent=2)
    print("Patching config file...")
    path.write_text(out + "\n", encoding="utf-8")
    print("Patch success")
except Exception as e:
    print(f"Skipping Waybar patch due to error: {e}", file=sys.stderr)
    import traceback
    traceback.print_exc()

