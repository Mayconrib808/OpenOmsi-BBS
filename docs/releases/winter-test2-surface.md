# Winter test 2: attach cloth to the actual loaded passenger surface

Based on native test 1 commit `9c78f21cf5a4269a68ff5e8819d127e42108a37c`.

The test 1 mannequin fit could preserve triangle lengths while leaving the
garment detached from a stock passenger. The new fit projects authored cloth
vertices onto nearby weighted human triangles. Barycentric attachments sample
the passenger's final skinned positions and normals, including arm/hand extras
and seated movement. The original garment topology, UVs and textures remain.

The fitting cache is bounded to 24 human/garment pairs; its key retains the
HumanType Arc so addresses cannot alias another model. Reused GPU instances
receive a new attachment and vertex upload. Equal source vertices share an
attachment across OBJ UV seams. Invalid or distant fits and distorted poses
are withheld, with model-specific diagnostics.

Tests cover real source garment loading, the existing rest-fit/pose bounds,
surface projection, removal of mannequin offsets, barycentric motion, missing
body data and invalid normals. The opt-in licensed-human numeric integration
test now exercises actual surface attachments as well.

This remains an explicitly opt-in test build, without visual approval. The
licensed stock-human integration test and in-game front/back/side screenshots,
walking, sitting, boarding, waving and paying remain pending. No licensed human
meshes are redistributed. Passing automated tests is not a gameplay sign-off.
