import json
import os
import re

from flask import Blueprint, request, jsonify

blueprint = Blueprint("editor", __name__)

robotsrc_path = os.path.join(os.getcwd(), "robotsrc")
if not os.path.exists(robotsrc_path):
    os.mkdir(robotsrc_path)

main_path = os.path.join(robotsrc_path, 'main.py')
with open(main_path, 'w') as f:
    f.write('# DO NOT DELETE\n')

blocks_path = os.path.join(robotsrc_path, 'blocks.json')


@blueprint.route('/')
def get_files():
    project_paths = [f for f in os.listdir(robotsrc_path) if
                     os.path.isfile(os.path.join(robotsrc_path, f)) and
                     (f.endswith('.py') or f.endswith('.xml') or f == 'blocks.json') and
                     f != 'main.py']

    def read_project(p):
        with open(os.path.join(robotsrc_path, p), 'r') as pf:
            content = pf.read()
        return {'filename': p, 'content': content}

    blocks = {}
    if os.path.exists(blocks_path):
        try:
            with open(blocks_path, 'r') as bf:
                blocks = json.load(bf)
        except:
            blocks = {}

    for k, v in [('requires', []), ('header', ""), ('footer', ""), ('blocks', [])]:
        blocks.setdefault(k, v)

    return jsonify({'main': main_path, 'blocks': blocks, 'projects': list(map(read_project, project_paths))})


@blueprint.route("/save/<string:filename>")
def save_file(filename):
    if '..' in filename or '/' in filename or filename.startswith('.'):
        return jsonify({'error': 'Invalid filename'}), 400

    dots = len(re.findall(r".", filename))
    if dots == 1:
        filepath = os.path.join(robotsrc_path, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(request.data.decode('utf-8'))
        return jsonify({'success': True})
    return jsonify({'error': 'Invalid filename format'}), 400


@blueprint.route("/delete/<string:filename>")
def delete_file(filename):
    if filename == "blocks.json":
        return jsonify({'error': 'Cannot delete blocks.json'}), 400
    if '..' in filename or '/' in filename or filename.startswith('.'):
        return jsonify({'error': 'Invalid filename'}), 400

    dots = len(re.findall(r".", filename))
    if dots == 1:
        filepath = os.path.join(robotsrc_path, filename)
        if os.path.exists(filepath):
            os.unlink(filepath)
        return jsonify({'success': True})
    return jsonify({'error': 'Invalid filename format'}), 400