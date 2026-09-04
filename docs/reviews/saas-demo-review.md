# SaaS example: giá trị demo và phạm vi nâng cấp

Ngày review: 2026-09-04. Phạm vi: cấu hình SaaS hiện tại, trải nghiệm browser, các tính năng Bean hiện có và kiểm chứng HTTP trên database tạm. Đây là review và đề xuất; chưa triển khai bản demo mới.

## Kết luận

**Chưa nên giới thiệu bản hiện tại như một SaaS demo hoàn chỉnh cho người dùng.** Nó có giá trị làm mẫu kỹ thuật về cách ly Project theo tenant, nhưng trải nghiệm hiện tại chủ yếu là ba bảng CRUD. Việc sửa README và đăng nhập đúng tài khoản giải quyết bước truy cập Project; chưa giải quyết tính nhất quán của Organisation, Membership và onboarding.

Nên nâng cấp thành **Team Workspace: quản lý dự án và theo dõi tiến độ riêng cho từng khách hàng**. Điểm khác biệt với Asana Lite là cùng một ứng dụng phục vụ nhiều tenant, với danh sách, dashboard, drill-down và thao tác đều tuân theo cùng phạm vi dữ liệu.

## Findings theo mức ưu tiên

### P1 — Organisation chưa có phạm vi đọc phù hợp với một demo đa tenant

`examples/saas/app.yaml:4` định nghĩa Organisation không có Policy, owner hay tenant. Kiểm chứng trên database tạm: admin tạo Organisation thành công; một request không đăng nhập tới `/api/views/organisation_list` nhận HTTP 200 cùng tên tổ chức đó.

Nếu Organisation là danh bạ công khai thì cần diễn đạt rõ và giới hạn trường công khai. Trong câu chuyện workspace riêng cho khách hàng, mặc định này không phù hợp: danh tính tổ chức cần được bảo vệ. Project vẫn có tenant predicate và các test cách ly Project đang pass; không suy rộng kết quả đó sang mọi Entity.

**Đề xuất:** chốt Organisation là hồ sơ workspace thuộc tenant hay dữ liệu quản trị hệ thống. Dùng Policy rõ ràng cho cả đọc và ghi. Bổ sung test anonymous và tenant B không đọc được hồ sơ tenant A. Không chỉ ẩn menu: phải bảo vệ cả generated View và Actions.

### P1 — Tenant owner được thấy form nhưng không thực hiện được một số nghiệp vụ

Organisation và Membership không có Policy ghi. `internal/action/action.go:792` cho phép administrator qua bước kiểm tra Policy, nhưng từ chối ghi không có Policy với các tài khoản khác. Vì vậy owner/editor mở Admin và thấy nút Add nhưng chưa có quyền tạo hai loại record này.

Kiểm chứng HTTP với owner/editor có tenant: `organisation_create` và `membership_create` đều trả 409. Project tạo và đọc được; Membership list đọc được. Lượt browser test trước mới bao phủ Membership list, chưa chứng minh Membership create/update/delete hoạt động.

**Đề xuất:** xác định thao tác dành cho owner và member, khai báo Action/Policy tương ứng, rồi chỉ cung cấp thao tác hợp lệ trên giao diện ứng dụng. Không cấp administrator cho tenant owner để che vấn đề này.

### P1 — Membership chưa phải mô hình quản lý thành viên của SaaS

`membership.user_id` chỉ là UUID; `membership.role` là dữ liệu nghiệp vụ. Session lấy roles và tenant từ `bean_user` (`internal/auth/auth.go:125`), không lấy từ Membership. Tạo Organisation cũng chưa tạo tenant context hay gắn user vào tổ chức. Project chỉ được gắn tenant từ context đăng nhập, không có quan hệ tới Organisation trong metadata này.

Kiểm chứng: một administrator có tenant tạo Membership với UUID user không tồn tại vẫn nhận HTTP 200. Điều này không chứng minh có lỗ hổng nâng quyền qua Membership; nó chứng minh Membership hiện chưa thực hiện lời hứa “quản lý thành viên và quyền trong tổ chức”.

**Đề xuất cho bản demo nhỏ:** provision tài khoản/tenant trước, ghi rõ giới hạn, chưa đưa form “quản lý quyền thành viên” vào hành trình chính. Nếu giữ roster, cần Policy, kiểm tra user hợp lệ và cách liên kết tổ chức rõ ràng. Luồng invite, đổi role, chuyển workspace và thu hồi quyền thực sự là một hạng mục identity riêng, cần thiết kế hợp đồng server; không thể chỉ thêm form.

### P2 — Thiếu một hành trình người dùng có kết quả hữu ích

Không có Page tại `/`, dữ liệu mẫu hay dashboard. Project chỉ có Name. Chưa có trạng thái, hạn hoàn thành, mô tả hoặc thao tác nghiệp vụ. User không thể trả lời “việc nào đang chạy, cần xử lý gì tiếp theo?”. Bảng và tiêu đề dùng UUID cũng làm trải nghiệm nghiêng về công cụ quản trị dữ liệu.

**Đề xuất:** có trang bắt đầu, tài khoản demo rõ ràng, dữ liệu đủ để xem tiến độ, tên Project làm link, và hành trình tạo → bắt đầu → hoàn thành → dashboard cập nhật.

### P2 — Role và bằng chứng demo còn hẹp

`tenant_member` cho member và owner cùng tập quyền đọc/ghi Project. Khác biệt trách nhiệm giữa hai vai trò chưa được thể hiện. Test hiện chứng minh tạo/đọc/cập nhật Project và từ chối truy cập chéo tenant; chưa bao phủ tenant-scoped aggregates, drill, export, quyền chuyển trạng thái, hay tính toàn vẹn Membership.

**Đề xuất:** đưa phân quyền vào hành động cụ thể. Ví dụ member tạo/cập nhật và hoàn thành Project; owner có thêm quyền archive/reopen. Không cần thêm role nếu không có sự khác biệt nghiệp vụ có thể demo và kiểm thử.

## Giá trị mang lại

| Đối tượng | Giá trị hiện tại | Giá trị của bản nâng cấp |
| --- | --- | --- |
| Developer xây ứng dụng nhiều khách hàng | Ví dụ nhỏ về tenant predicate trên View/Action | Một mẫu xuyên suốt: UI, API, thống kê và thao tác cùng giữ phạm vi tenant, không viết lại bộ lọc ở từng nơi |
| Người quản lý một workspace | Tạo một record có tên | Biết số dự án đang chạy, mở nhóm cần xử lý, cập nhật tiến độ và xem kết quả ngay |
| Người đánh giá Bean | Thấy CRUD tự sinh | Thấy thay đổi metadata tạo ra giao diện, workflow, API và các kiểm tra có thể lặp lại |

Thông điệp demo đề xuất: **“Một cấu hình ứng dụng, nhiều workspace độc lập; từ dashboard tới thao tác trên record đều giữ đúng phạm vi khách hàng.”** Đây là giá trị có thể quan sát được, thay vì chỉ đọc một lời hứa về tenant isolation.

## Tận dụng tính năng hiện có có chọn lọc

| Tính năng | Cách dùng trong SaaS | Giá trị quan sát được |
| --- | --- | --- |
| Named View Displays | Project list/detail; JSON và CSV trên cùng query | UI/API/export dùng cùng Policy; tên dự án dẫn tới detail |
| Aggregate Views + metric/chart | Tổng số Project, phân bố theo trạng thái | Dashboard của A chỉ tính dữ liệu của A |
| Page filters + typed drill | Lọc status; bấm cột chart để mở danh sách tương ứng | User đi từ số liệu tới đúng tập record và hành động tiếp |
| Lifecycle + domain Actions | Planned → Active → Completed; archive/reopen có quyền rõ ràng | Trạng thái hợp lệ, generic update không bỏ qua quy trình |
| Rule + TestSuite | Chỉ thêm guard có nhu cầu thật, ví dụ lý do archive bắt buộc | Lỗi nghiệp vụ cụ thể, kiểm chứng được bằng case dương/âm |
| Page sections + responsive Panels | Trang tổng quan, bộ lọc và danh sách trong các khu vực có mục đích | Dễ đọc trên desktop/mobile, không cần CSS riêng cho example |
| Menu `profile: workspace`, `variant: line` | Overview / Projects, với mục owner-only khi thật sự có chức năng | Điều hướng rõ ràng và theo Policy; hai mức là đủ |
| Webform + bound context | Tạo/cập nhật Project qua Action trên trang ứng dụng | User thường không cần role editor chỉ để làm việc |
| AdminResource | Label là name, ẩn ID khỏi list, search/filter/sort và Actions | Admin hỗ trợ vận hành thay vì làm toàn bộ sản phẩm |
| Theme và shared Shell | Tên workspace/demo, accent nhất quán, light/dark | Hoàn thiện trải nghiệm với hệ thống UI sẵn có |

Không thêm tree, attachments, dynamic menu placement hoặc nhiều biểu đồ chỉ để trình diễn số lượng tính năng. Scoped Menu tổ chức điều hướng theo record; **không phải cơ chế chuyển tenant**. Context bảo mật vẫn phải đến từ session/server, không nhận tenant tùy ý từ query string.

## Những giới hạn cần tính vào kế hoạch

- `LocalRegistration` hiện cấp một default role và không cho client gửi tenant. Nó chưa cung cấp luồng đăng ký kèm tạo Organisation/Membership và chọn workspace. Không quảng bá self-service signup chỉ bằng cách thêm trang đăng ký.
- `DemoSeed` hiện chạy với một actor và tenant tổng hợp (`internal/demoseed/demoseed.go:284`). Nó chưa tạo bộ tài khoản đăng nhập cho hai tenant. Với demo A/B, dùng setup script thuộc example: tạo tài khoản rồi seed bằng session/Actions của từng tenant. Không nới Policy và không chèn business data trực tiếp bằng SQL để seed dễ hơn.
- Bản nâng cấp cần thêm trường trạng thái/metadata. Dùng database demo mới hoặc một kế hoạch migration additive được kiểm chứng; không xóa hay reset database SaaS người dùng đang mở.
- Hiện chưa đủ nền tảng để hứa billing, invite email, multi-workspace membership hay workspace switcher. Những khả năng đó nên có phạm vi và test riêng.

## Phạm vi nâng cấp đề xuất

### 1. Sửa hợp đồng dữ liệu và hành trình khởi động

Bảo vệ Organisation; xác định rõ Membership có vai trò gì. Tạo setup có thể lặp lại cho hai tenant và owner/member của mỗi tenant. Trang `/` có giới thiệu ngắn và đường đăng nhập đúng; sau đăng nhập vào trang workspace có nội dung. Giữ tài khoản system admin dành cho quản trị hệ thống.

### 2. Làm một lát cắt sản phẩm đủ sâu

Project có name, description, status và một trường deadline nếu dùng trong hành trình. Dashboard nhỏ, danh sách có filter, detail và Actions cần thiết. Dữ liệu mẫu A/B khác nhau để thấy rõ phạm vi. Người dùng làm việc qua Page/Webform; Admin dành cho vận hành. Navigation chỉ chứa màn hình đã hoạt động.

### 3. Chứng minh lợi thế của Bean

Thay đổi trạng thái một Project, quay lại dashboard thấy count đổi; drill ra đúng tập record. Đăng nhập B, thấy danh sách/count khác, không mở được URL của A. CSV/JSON cũng giữ phạm vi đó. Test cùng một câu chuyện qua UI và API.

Có thể tách source thành `access.yaml`, `projects.yaml`, `workspace.yaml` khi nâng cấp để giữ definition theo trách nhiệm. Phần identity thực sự nằm ngoài phạm vi ba bước này cho tới khi có quyết định thiết kế riêng.

## Tiêu chí đủ tốt để đưa cho user

1. Làm theo một hướng dẫn setup là có dữ liệu và tài khoản dùng được; không cần tự đoán tenant UUID hay mở màn hình 404.
2. Trong một lượt demo khoảng 3–5 phút, user tạo/chuyển trạng thái Project và quan sát dashboard cập nhật.
3. Mỗi nút trong hành trình chính thực hiện được với đúng role, hoặc bị giới hạn có giải thích; không có form Membership giả vờ cấp quyền.
4. A/B cách ly ở list, detail, Action, metric, drill và export; Organisation cũng có phạm vi đã thống nhất.
5. Light/dark và mobile dùng được; tên record thay cho UUID ở các vị trí chính.
6. Setup/test lặp lại được; `make check` và `make build` pass. Không gọi đây là production SaaS starter khi onboarding/identity chưa được triển khai.

## Bằng chứng và nguồn trong repo

- SaaS hiện có 5 definitions; `bean app validate --file examples/saas/app.yaml` pass.
- `e2e/saas.spec.ts`: test tenant-owner Admin vừa thêm và test member-only isolation đều pass. Gate gần nhất trước review: 90 frontend tests, 22 browser journeys, build pass. Review này không thay đổi runtime.
- HTTP audit chạy trên database tạm, được dọn sau khi chạy: admin tạo Organisation 200; anonymous đọc tên Organisation 200; tenant owner tạo Organisation/Membership 409; tenant administrator tạo Membership có user UUID không tồn tại 200; anonymous Organisation update/delete 409. Không suy diễn rằng anonymous có quyền ghi.
- `examples/saas/app.yaml:4`: Organisation; `:10`: Membership; `:23`: Project; `:31`: Policy; `:37`: JSON View.
- `internal/action/action.go:792`: authorization mặc định cho Action; `internal/auth/auth.go:125`: nguồn roles/tenant của session.
- `docs/capabilities.md`: năng lực đã có; `docs/definitions.md`: View displays, AdminResource, registration, Page composition.
- `examples/ats/app.yaml`: lifecycle, metric/filter/drill và seed; `examples/asana/pages.yaml`: workspace và Page sections; `examples/books/model.yaml`: named record navigation, Menu và AdminResource.
- `docs/goals/015.md`: tenant-scoped aggregate dashboard cho SaaS đã từng được ghi là phần phát triển tiếp, chưa thuộc example hiện tại.

## Kết quả triển khai sau review

Đã thay fixture cũ bằng Team Workspace: 46 definitions, Organisation có tenant policy, Project lifecycle với quyền owner/member, dashboard/filter/drill, form tạo/đổi tên, Settings và JSON/CSV cùng phạm vi. Setup tạo hai workspace với 6/3 dự án và bốn tài khoản; từ chối ghi đè DB. Membership cũ được bỏ vì không điều khiển identity.

Đủ để demo workflow và giá trị metadata của Bean: owner hoàn thành một hành trình dự án, dashboard phản ánh thay đổi; member bị giới hạn đúng quyền; tenant B không truy cập dữ liệu A qua UI, Action, tổng hợp hay export. Ba SaaS browser/API journeys và semantic contracts pass; kiểm tra light/dark/mobile hoàn tất. `make check` pass 90 frontend tests, 23 browser journeys; `make build` pass.

Giới hạn còn rõ ràng: tài khoản/tenant được provision trước, chưa có onboarding, invitations hay billing. Xem `examples/saas/README.md` cho walkthrough; dùng DB mới, không migrate phá huỷ dữ liệu SaaS cũ. Các findings phía trên mô tả bản trước nâng cấp.
