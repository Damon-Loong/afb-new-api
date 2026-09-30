# Aperture homepage design QA

## Visual targets
- Dark: C:/Users/l/.codex/generated_images/01a0eb2f-867e-7182-87f4-4f519ae4a165/exec-9ebadb7c-a3ec-4088-ae1d-1111af55290e.png
- Light: C:/Users/l/.codex/generated_images/01a0eb2f-867e-7182-87f4-4f519ae4a165/exec-c390cf5f-210e-4560-8511-6a9bafdd776f.png
- Implementation: http://127.0.0.1:4173/
- Captures: deploy-packages/aperture-dark-qa.png, aperture-light-qa-final.png, aperture-mobile-qa.png
- Desktop CSS viewport1488x1056; source1488x1056 approximately, generated clean backgrounds1489x1056. Browser screenshot1488x1056. Images were opened together in the same tool output for comparisons, with no resizing of the implementation. Header intentionally retains actual shared implementation, per user instruction.

## Comparison history
1. Initial light comparison found P2 title too small and action/content block about15px too low. Increased title scale6.05vw to6.3vw, adjusted leading1.18 to1.12, top padding18.5vw to18.2vw, and enlarged action text/icons. Also found footer join abrupt; added a48px image-edge mask to blend into footer surface.
2. Post-fix light and dark captures compared to source. Title/body hierarchy, section line position, endpoint/action placement, architectural step and floor composition now closely follow the references. Shared header differences are explicitly intended. Minor raster texture and font rasterization differences remain; this is not a pixel-identical screenshot clone.

## Five fidelity surfaces
- Typography: existing system font, weight800 headline, two Chinese lines, blue accent. Original translation retained for other locales; traditional Chinese title rendered separately. UI text remains selectable and controls are real components.
- Layout: full-width body; reference5% horizontal inset and approximately18% top inset. Shared header is unmodified and still max1440px. Normal responsive layout replaces exact reference positioning at small widths.
- Color: dark navy/off-white and daylight navy/blue themes, separate reference-derived background assets. Decorative brightness animation never affects UI text.
- Assets: imagegen removed UI from each source composition. Original approved logo and provider icon library preserved. No invented vector artwork. Backgrounds retain architecture and reflected floor; generation yields minor texture differences.
- Copy: headline, subtitle, address, endpoint carousel, actions and provider/capability summary retained. Five featured providers plus30+ summary follow selected reference.

## Functional checks
- Copy button shows success toast.
- Documentation action opens/interface-docs with real API directory and content.
- API key action retains/console target and existing auth flow; local proxy session produced401/login-expired state (no authentication code changed).
- Theme selection switches matching background and text styles.
- Mobile390x844: no horizontal overflow; address/carousel wraps into two rows; actions and summary remain readable.
- Wide2557x1430: main2557.6px, header1440px, no horizontal overflow. Browser override reset afterward.
- Animation sampled brightness0.981575 and1.07921. Reduced-motion disables animation through CSS.
- AST check: requests, notice logic, custom content loading, address choice, copy handler and endpoint timer unchanged.
- Production build passed55.63s; existing dependency warnings about Browserslist, chunk size and lottie eval persist. Browser has existing React18/legacy render warnings and local proxy401 auth warning; no artwork-specific error observed.

## Follow-up polish
- P3: generated background texture is not pixel-identical to source; browser font rasterization differs slightly from image mock.
- P3: backgrounds are1489px wide, so very high-DPI ultra-wide displays will interpolate the raster. UI remains native-resolution.

final result: passed

## Requested motion and spacing adjustment
- Restored existing Git sweep-shine title animation (4 seconds).
- Desktop content raised about100–125px; compensating bottom padding preserves background height and registration. Mobile title begins at112px.
- Added6-second moving image-mask light pass; background geometry and shared header unchanged.
- Inspected dark and light desktop, plus390x844 mobile: no horizontal overflow or header overlap. Sampled mask positions change from negative49% to139%.
- Server production image build passed; existing behavior check passed.

## Four-second light and footer polish
- Travel duration4s. Light theme uses brightness1.18 and lighten blending; no dimming pass.
- Homepage-only footer overlaps unused background floor space. Checked1435px desktop,2551px wide and390px mobile; footer follows summary with no overlap or horizontal overflow.
- Existing homepage behavior check passed. Shared footer component and business logic unchanged.

## Foreground sizing and one-time title shine
- Hero scales to94% desktop,97% mobile; action height reduced further. Desktop content moves24px up,1800px+ moves40px up.
- Background rules unchanged; action padding preserves scene height.
- Title plays once per mount and finishes in solid currentColor, avoiding a frozen highlight on the last character.
- Browser checked1435px,2551px and390px. No horizontal overflow; mobile buttons44.62px high. Wide title top195px. Title final gradient verified solid.
- Existing homepage behavior check passed.
