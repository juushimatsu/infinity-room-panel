const { app, BrowserWindow } = require("electron");
const path = require("path");
const { spawn } = require("child_process");
const fs = require("fs");

let mainWindow = null;
let backendProcess = null;

// Project root is one level up from electron/ directory
const projectRoot = path.resolve(path.join(__dirname, ".."));

function findBinaryPath() {
  const binaryName =
    process.platform === "win32" ? "audiobot-panel.exe" : "audiobot-panel";
  const possiblePaths = [
    path.join(projectRoot, "backend", binaryName),
    path.join(projectRoot, "backend", "build", binaryName),
    path.join(process.resourcesPath || "", "backend", binaryName),
  ];

  for (const p of possiblePaths) {
    try {
      fs.accessSync(p);
      return p;
    } catch {}
  }
  return null;
}

async function startBackend() {
  const binaryPath = findBinaryPath();
  if (!binaryPath) {
    const { dialog } = require("electron");
    dialog.showErrorBox(
      "Backend not found",
      `Could not find the Go backend binary at:\n${path.join(projectRoot, "backend", "audiobot-panel.exe")}\n\nPlease build it first:\ngo build -o backend/audiobot-panel.exe ./backend/`,
    );
    app.quit();
    return null;
  }

  // Find a free port
  const net = require("net");
  const server = net.createServer();
  await new Promise((resolve) => {
    server.listen(0, resolve);
  });
  const port = server.address().port;
  server.close();

  console.log("[electron] Starting backend on port", port);
  console.log("[electron] Project root:", projectRoot);

  // Spawn backend with CWD set to project root so it can find frontend/build
  backendProcess = spawn(binaryPath, [], {
    cwd: projectRoot,
    env: {
      ...process.env,
      PORT: String(port),
      ELECTRON_MODE: "1",
    },
    stdio: ["pipe", "pipe", "pipe"],
  });

  backendProcess.stdout.on("data", (data) => {
    const str = data.toString();
    console.log("[backend]", str.trim());
    if (str.includes("PORT=")) {
      createWindow(port);
    }
  });

  backendProcess.stderr.on("data", (data) => {
    console.error("[backend]", data.toString().trim());
  });

  backendProcess.on("error", (err) => {
    console.error("Failed to start backend:", err);
  });

  backendProcess.on("exit", (code) => {
    console.log(`Backend exited with code ${code}`);
  });

  // Fallback: create window after timeout
  setTimeout(() => {
    if (!mainWindow) {
      createWindow(port);
    }
  }, 3000);

  return port;
}

function createWindow(port) {
  if (mainWindow) return;

  mainWindow = new BrowserWindow({
    width: 900,
    height: 750,
    title: "AudioBot Panel",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      nodeIntegration: false,
      contextIsolation: true,
    },
  });

  mainWindow.loadURL(`http://localhost:${port}`);

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

app.whenReady().then(startBackend);

app.on("window-all-closed", () => {
  if (backendProcess) {
    backendProcess.kill();
    backendProcess = null;
  }
  app.quit();
});

app.on("before-quit", () => {
  if (backendProcess) {
    backendProcess.kill();
    backendProcess = null;
  }
});
