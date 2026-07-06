# V-CoPanel Offline Assets Repository (`pc-assets/`)

To prevent repository bloat and speed up git operations, all heavy offline zip archives and installers in this folder are ignored by Git.

If you are developing V-CoPanel from source, you must populate the subdirectories in this folder with their corresponding runtime zip archives.

## How to populate this directory:
You have two options:
1. **Copy from Releases (Recommended)**: Download the latest pre-compiled release package from [GitHub Releases](https://github.com/vjects/V-CoPanel-Windows/releases), extract it, and copy its entire `pc-assets/` folder here.
2. **Download Manually**: Download the official Windows 64-bit packages directly from the vendor websites and place them inside the folders with the exact filenames.

---

### Required Assets Structure:

#### 1. `pc-assets/php/`
Contains the raw Windows 64-bit PHP thread-safe zip packages:
- `php-8.2.32-win64.zip` (or equivalent PHP 8.2 Windows zip)
- `php-8.3.32-win64.zip` (or equivalent PHP 8.3 Windows zip)
- `php-8.4.23-win64.zip` (or equivalent PHP 8.4 Windows zip)
- `php-8.5.8-win64.zip` (or equivalent PHP 8.5 Windows zip)

#### 2. `pc-assets/node/`
Contains the Windows 64-bit Node.js zip packages:
- `node-v20.18.3-win-x64.zip` (or equivalent Node v20 zip)
- `node-v22.14.0-win-x64.zip` (or equivalent Node v22 zip)

#### 3. `pc-assets/mariadb/`
Contains the Windows 64-bit MariaDB server zip package:
- `mariadb-11.4.5-winx64.zip` (or equivalent MariaDB 11.4 zip)

#### 4. `pc-assets/redis/`
Contains the Windows 64-bit Redis zip package (e.g. tporadowski/redis build):
- `redis-5.0.14.1-winx64.zip`

#### 5. `pc-assets/mailpit/`
Contains the Mailpit Windows executable package (extracted or zipped):
- `mailpit-windows-amd64.zip`

#### 6. `pc-assets/phpmyadmin/`
Contains the phpMyAdmin zip package:
- `phpMyAdmin-5.2.2-all-languages.zip`

#### 7. `pc-assets/composer/`
Contains the Composer installer/executable:
- `composer.phar`

#### 8. `pc-assets/vcredist/`
Contains the Microsoft Visual C++ Redistributable offline installer:
- `VC_redist.x64.exe`
