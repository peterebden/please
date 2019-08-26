def add_module_dir_to_sys_path(dirname):
    """Adds the given dirname to sys.path if it's nonempty."""
    if dirname:
        sys.path = sys.path[:1] + [os.path.join(sys.path[0], dirname)] + sys.path[1:]
        sys.meta_path.insert(0, ModuleDirImport(dirname))


def run():
    # Add .bootstrap dir to path, after the initial pex entry
    sys.path = sys.path[:1] + [os.path.join(sys.path[0], '.bootstrap')] + sys.path[1:]
    if not ZIP_SAFE:
        try:
            fuse = setup_fuse()
            return interact(main)
        except OSError as err:
            import logging
            logging.warning('Unable to set up FUSE filesystem: %s', err)
        with explode_zip()():
            add_module_dir_to_sys_path(MODULE_DIR)
            return interact(main)
    else:
        add_module_dir_to_sys_path(MODULE_DIR)
        sys.meta_path.append(SoImport())
        return interact(main)


if __name__ == '__main__':
    if 'PEX_PROFILE_FILENAME' in os.environ:
        with profile(os.environ['PEX_PROFILE_FILENAME'])():
            result = run()
    else:
        result = run()
    sys.exit(result)
