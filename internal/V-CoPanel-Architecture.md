# V-CoPanel Enterprise Engine

This project is currently being managed and hosted by **V-CoPanel** (A portable, zero-dependency local development engine).

## What You Need To Know
- **Local Server**: V-CoPanel uses isolated, native runtimes (like PHP, Node, Go) installed locally inside the V-CoPanel workspace. It does not use Docker or global system installations.
- **Environment (.env)**: V-CoPanel has injected a "Sandbox Block" at the end of your `.env` file containing local database (MariaDB 11+), Redis, and Mailpit credentials. Please do not modify this block manually.
- **Shims**: V-CoPanel automatically places `.bat` and `.cmd` shim files (like `php.bat`, `composer.bat`, `npm.cmd`) in this project's root. These shims intercept commands to use the portable V-CoPanel runtimes automatically.
- **Runtime File**: The `.vcopanel-runtime.json` file tracks the framework and port allocations.

## Agent Instructions
If you are an AI Agent assisting the user in this directory:
1. You can run standard framework commands (like `php artisan serve` or `npm run dev`), and the local `.bat` shims will automatically route them to the correct isolated runtime.
2. The database is pre-configured and accessible via `127.0.0.1:3306`.
3. Do not modify the `# --- BEGIN V-COPANEL SANDBOX BLOCK ---` in the `.env` file.
