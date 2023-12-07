version_settings(constraint='>=0.33.6')

# resources
k8s_yaml([
    'deploy/dispatcher.yaml',
    'deploy/pubsub.yaml',
])

# dispatcher
k8s_resource('dispatcher', port_forwards='0.0.0.0:5888:5000',labels=['backend'])
docker_build(
    'dispatcher',
    context='.',
    dockerfile='apps/dispatcher/cmd/Dockerfile.dev',
    only=[
        'apps/dispatcher',
        'libs/go',
        'go.mod',
        'go.sum',
    ],
    live_update=[
        sync('apps/dispatcher/', '/code/apps/dispatcher'),
        sync('libs/', '/code/libs'),
        run(
            'go mod tidy',
            trigger=['apps/dispatcher/']
        )
    ]
)

# https://docs.tilt.dev/api.html#modules.config.main_path
tiltfile_path = config.main_path

# https://github.com/bazelbuild/starlark/blob/master/spec.md#print
print("""
Starting local gcp doc ai services
""".format(tiltfile=tiltfile_path))
