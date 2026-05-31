# Project Explanation: Shepherd

## Overview

The 'shepherd' project is a web-based control system designed for a Raspberry Pi robot, specifically for a robotics competition. It features a Flask web server that enables users to upload, edit, and execute Python code directly on the robot.

The architecture is built around a main Flask application (`shepherd/__init__.py`) which orchestrates several modular blueprints:
-   **`run`**: Manages the execution of user code in a separate, long-running subprocess. It uses a named pipe (FIFO) for signaling user code and a 'reaper' mechanism to terminate it after a defined period (e.g., `ROUND_LENGTH` of 180 seconds).
-   **`upload`**: Handles receiving user code (as `.py` or `.zip` files), replacing the current code, and triggering the `run` blueprint to restart the execution cycle with the new code.
-   **`editor`**: Provides a JSON API for a web-based file editor, allowing file system operations (list, read, write, delete) on the user's robot code.
-   **`pyls`**: (Inferred) Likely provides Python Language Server capabilities, possibly over WebSockets, to enhance the editing experience within the web editor.

The system is designed for specific hardware interaction, engaging with Raspberry Pi GPIO pins for a physical start button and relying on an external `robot` library (expected at `/home/pi/robot`) for hardware control. This project aims to provide a complete, self-contained development and execution environment for the robot within the context of a competition.

## File Explanations

### `app.py`
This is the main entry point of the application. It imports the core Flask `app` object from the `shepherd` package and serves it using a gevent WSGI server with WebSocket support, highlighting the application's asynchronous nature.

### `shepherd/__init__.py`
This file serves as the heart of the Flask application. It's responsible for initializing the app, configuring it, registering all the blueprints (modules) that define the application's routes and logic, and setting up hardware-specific features like GPIO button handling for the Raspberry Pi. Key components include the Flask app instance, WebSocket integration, GPIO event detection, and blueprint registration.

### `shepherd/blueprints/run/__init__.py`
This blueprint manages the entire lifecycle of the user's code. It executes the code as a separate process, facilitates communication via a named pipe (FIFO), and employs a state machine to track its status. It also contains critical 'reaper' and 'butcher' functions for terminating user code, either on command or after a timeout.

### `shepherd/blueprints/upload/__init__.py`
This blueprint handles the submission of user code. It processes and validates uploaded `.py` or `.zip` files, replaces existing code, and is closely integrated with the `run` blueprint to stop any currently running process and start the newly uploaded code. It also includes calls to an external `robot.reset` module.

### `shepherd/blueprints/editor/__init__.py`
This file provides the backend functionality for a web-based file editor. It contains API endpoints for various file system operations such as listing, creating, reading, updating, and deleting files within the user's designated code directory (`SHEPHERD_USER_CODE_PATH`). This enables users to manage their project files directly through the web interface.

### `shepherd/competition.py`
A straightforward configuration file that defines a crucial parameter for the competition mode: the duration of a round in seconds. This `ROUND_LENGTH` constant is utilized by the `run` blueprint to automatically terminate user code after the specified period.

## Other Files and Directories
Many other files and directories exist within the project (e.g., static assets, templates, documentation, specific editor bundles, git configurations, build files, etc.) which contribute to the overall functionality and deployment but were not detailed in this initial analysis. These typically support the web interface, provide project metadata, or are part of the development/build pipeline.
